package phtrace

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/sdk/resource"
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const testIngestKey = "test-ingest-key-not-a-secret"

// otlpStub is an in-process OTLP/gRPC collector that requires the same
// `authorization` metadata the ClickStack collector does.
type otlpStub struct {
	collectortrace.UnimplementedTraceServiceServer
	collectormetrics.UnimplementedMetricsServiceServer
	spans   atomic.Int64
	metrics atomic.Int64
}

func (s *otlpStub) Export(ctx context.Context, req *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	if err := requireKey(ctx); err != nil {
		return nil, err
	}
	for _, rs := range req.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			s.spans.Add(int64(len(ss.GetSpans())))
		}
	}
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

type metricsStub struct{ *otlpStub }

func (m metricsStub) Export(ctx context.Context, req *collectormetrics.ExportMetricsServiceRequest) (*collectormetrics.ExportMetricsServiceResponse, error) {
	if err := requireKey(ctx); err != nil {
		return nil, err
	}
	for _, rm := range req.GetResourceMetrics() {
		for _, sm := range rm.GetScopeMetrics() {
			m.metrics.Add(int64(len(sm.GetMetrics())))
		}
	}
	return &collectormetrics.ExportMetricsServiceResponse{}, nil
}

func requireKey(ctx context.Context) error {
	md, _ := metadata.FromIncomingContext(ctx)
	if got := md.Get("authorization"); len(got) != 1 || got[0] != testIngestKey {
		return status.Error(codes.Unauthenticated, "missing or wrong ingest key")
	}
	return nil
}

// startStub serves the stub on 127.0.0.1. With tlsCert it serves TLS only, so
// a plaintext client cannot complete a request.
func startStub(t *testing.T, tlsCert *tls.Certificate) (*otlpStub, string) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	var opts []grpc.ServerOption
	if tlsCert != nil {
		opts = append(opts, grpc.Creds(credentials.NewServerTLSFromCert(tlsCert)))
	}
	srv := grpc.NewServer(opts...)
	stub := &otlpStub{}
	collectortrace.RegisterTraceServiceServer(srv, stub)
	collectormetrics.RegisterMetricsServiceServer(srv, metricsStub{stub})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return stub, lis.Addr().String()
}

// selfSignedCert returns a server certificate for 127.0.0.1 and the path of a
// PEM file holding it, to be trusted as the CA.
func selfSignedCert(t *testing.T) (*tls.Certificate, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "phtrace-test-collector"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("cert: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("key pair: %v", err)
	}
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caPath, certPEM, 0o600); err != nil {
		t.Fatalf("write ca: %v", err)
	}
	return &pair, caPath
}

func exportOneSpan(t *testing.T, cfg Config) error {
	t.Helper()
	tp, _, err := buildTracerProvider(context.Background(), cfg.withDefaults(), resource.Empty())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer func() { _ = tp.Shutdown(ctx) }()
	_, span := tp.Tracer("phtrace-test").Start(ctx, "export-probe")
	span.End()
	return tp.ForceFlush(ctx)
}

func exportOneMetric(t *testing.T, cfg Config) error {
	t.Helper()
	mp, _, err := buildMeterProvider(context.Background(), cfg.withDefaults(), resource.Empty())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer func() { _ = mp.Shutdown(ctx) }()
	counter, err := mp.Meter("phtrace-test").Int64Counter("export_probe_total")
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	counter.Add(ctx, 1)
	return mp.ForceFlush(ctx)
}

func TestTraceExport_TLSWithIngestHeader(t *testing.T) {
	cert, caPath := selfSignedCert(t)
	stub, addr := startStub(t, cert)
	cfg := Config{
		ServiceName: "phtrace-test",
		Endpoint:    addr,
		Insecure:    false,
		CACertFile:  caPath,
		// A positive DialTimeout used to force insecure transport credentials.
		DialTimeout: 2 * time.Second,
		Headers:     map[string]string{"authorization": testIngestKey},
	}

	if err := exportOneSpan(t, cfg); err != nil {
		t.Fatalf("export over TLS with the ingest key: %v", err)
	}
	if got := stub.spans.Load(); got != 1 {
		t.Fatalf("collector received %d spans, want 1", got)
	}
}

func TestMetricExport_TLSWithIngestHeader(t *testing.T) {
	cert, caPath := selfSignedCert(t)
	stub, addr := startStub(t, cert)
	cfg := Config{
		ServiceName: "phtrace-test",
		Endpoint:    addr,
		CACertFile:  caPath,
		DialTimeout: 2 * time.Second,
		Headers:     map[string]string{"authorization": testIngestKey},
	}

	if err := exportOneMetric(t, cfg); err != nil {
		t.Fatalf("metric export over TLS with the ingest key: %v", err)
	}
	// ForceFlush and the deferred Shutdown each export the cumulative counter.
	if got := stub.metrics.Load(); got < 1 {
		t.Fatalf("collector received %d metrics, want at least 1", got)
	}
}

func TestExport_WithoutIngestHeaderIsRejected(t *testing.T) {
	cert, caPath := selfSignedCert(t)
	stub, addr := startStub(t, cert)
	cfg := Config{ServiceName: "phtrace-test", Endpoint: addr, CACertFile: caPath, DialTimeout: 2 * time.Second}

	if err := exportOneSpan(t, cfg); err == nil {
		t.Fatal("span export without the ingest key succeeded, want the collector to refuse it")
	}
	if err := exportOneMetric(t, cfg); err == nil {
		t.Fatal("metric export without the ingest key succeeded, want the collector to refuse it")
	}
	if stub.spans.Load() != 0 || stub.metrics.Load() != 0 {
		t.Fatalf("collector accepted data without the key: spans=%d metrics=%d", stub.spans.Load(), stub.metrics.Load())
	}
}

func TestExport_UntrustedCertificateIsRejected(t *testing.T) {
	cert, _ := selfSignedCert(t)
	_, otherCA := selfSignedCert(t)
	stub, addr := startStub(t, cert)
	cfg := Config{
		ServiceName: "phtrace-test",
		Endpoint:    addr,
		CACertFile:  otherCA,
		DialTimeout: 2 * time.Second,
		Headers:     map[string]string{"authorization": testIngestKey},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	tp, _, err := buildTracerProvider(ctx, cfg.withDefaults(), resource.Empty())
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer func() { _ = tp.Shutdown(context.Background()) }()
	_, span := tp.Tracer("phtrace-test").Start(ctx, "export-probe")
	span.End()
	if err := tp.ForceFlush(ctx); err == nil {
		t.Fatal("export to a collector with an untrusted certificate succeeded")
	}
	if stub.spans.Load() != 0 {
		t.Fatal("collector received a span over an untrusted TLS session")
	}
}

func TestExport_InsecureWithIngestHeader(t *testing.T) {
	stub, addr := startStub(t, nil)
	cfg := Config{
		ServiceName: "phtrace-test",
		Endpoint:    addr,
		Insecure:    true,
		DialTimeout: 2 * time.Second,
		Headers:     map[string]string{"authorization": testIngestKey},
	}

	if err := exportOneSpan(t, cfg); err != nil {
		t.Fatalf("plaintext export with the ingest key: %v", err)
	}
	if err := exportOneMetric(t, cfg); err != nil {
		t.Fatalf("plaintext metric export with the ingest key: %v", err)
	}
	if stub.spans.Load() != 1 || stub.metrics.Load() < 1 {
		t.Fatalf("collector got spans=%d metrics=%d, want 1 and at least 1", stub.spans.Load(), stub.metrics.Load())
	}
}

func TestBuild_MissingCACertFileFailsWithoutLeakingHeaders(t *testing.T) {
	cfg := Config{
		ServiceName: "phtrace-test",
		Endpoint:    "127.0.0.1:1",
		CACertFile:  filepath.Join(t.TempDir(), "absent.pem"),
		DialTimeout: time.Second,
		Headers:     map[string]string{"authorization": testIngestKey},
	}
	_, _, err := buildTracerProvider(context.Background(), cfg.withDefaults(), resource.Empty())
	if err == nil {
		t.Fatal("expected an error for an unreadable CA file")
	}
	if strings.Contains(err.Error(), testIngestKey) {
		t.Fatalf("error leaks the header value: %v", err)
	}
}

func TestParseOTLPHeaders(t *testing.T) {
	got, err := parseOTLPHeaders(" Authorization = abc%3D%3D , x-scope-orgid=tenant%201 ")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := map[string]string{"authorization": "abc==", "x-scope-orgid": "tenant 1"}
	if len(got) != len(want) || got["authorization"] != want["authorization"] || got["x-scope-orgid"] != want["x-scope-orgid"] {
		t.Fatalf("got %v, want %v", got, want)
	}
	if got, err := parseOTLPHeaders("  "); err != nil || len(got) != 0 {
		t.Fatalf("blank: got %v, %v", got, err)
	}
}

func TestParseOTLPHeaders_RejectsMalformedWithoutPrintingValues(t *testing.T) {
	const secret = "s3cr3t-value"
	for _, raw := range []string{
		"authorization",
		"=" + secret,
		"authorization=",
		"bad key=" + secret,
		"authorization=" + secret + "%0Aevil",
		"authorization=" + secret + "%zz",
		"authorization=" + secret + ",authorization=" + secret,
	} {
		_, err := parseOTLPHeaders(raw)
		if err == nil {
			t.Errorf("%q: expected an error", strings.ReplaceAll(raw, secret, "<value>"))
			continue
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("error leaks the value: %v", err)
		}
	}
}

func TestFromEnv_ReadsIngestHeadersAndCertificate(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "authorization="+testIngestKey)
	t.Setenv("OTEL_EXPORTER_OTLP_CERTIFICATE", "/etc/otel/ca.pem")
	cfg := FromEnv()
	if cfg.Headers["authorization"] != testIngestKey || cfg.CACertFile != "/etc/otel/ca.pem" {
		t.Fatalf("FromEnv did not load headers/certificate: headers=%d ca=%q", len(cfg.Headers), cfg.CACertFile)
	}
	if err := ensureRequired(Config{ServiceName: "s", Endpoint: "e", SamplingRatio: 1, DialTimeout: time.Second, Headers: cfg.Headers}); err != nil {
		t.Fatalf("valid headers rejected: %v", err)
	}
}

func TestFromEnv_MalformedHeadersFailInitValidation(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "authorization")
	cfg := FromEnv(WithServiceName("s"), WithEndpoint("e")).withDefaults()
	err := ensureRequired(cfg)
	if err == nil || !strings.Contains(err.Error(), "OTEL_EXPORTER_OTLP_HEADERS") {
		t.Fatalf("expected a header configuration error, got %v", err)
	}
}
