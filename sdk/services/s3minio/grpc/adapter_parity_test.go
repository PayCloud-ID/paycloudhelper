package grpc

import (
	"context"
	"errors"
	"testing"

	"bitbucket.org/paycloudid/paycloudhelper/sdk/services/s3minio/helper"
	"bitbucket.org/paycloudid/paycloudhelper/sdk/services/s3minio/pb"
	gogrpc "google.golang.org/grpc"
)

// ============================================================================
// Adapter Parity Validation Tests
// Validates the gRPC adapter correctly implements all helper.Client methods
// and properly maps between helper DTOs and pb types.
// ============================================================================

// --- Interface assertion: gRPC Client must satisfy full helper.Client ---

func TestGRPCClientFullInterfaceSatisfaction(t *testing.T) {
	t.Parallel()

	// Compile-time: *Client must implement helper.Client (all 7 interfaces)
	var _ helper.Client = (*Client)(nil)

	// Also verify individual interface satisfaction
	var c *Client
	var _ helper.Downloader = c
	var _ helper.Uploader = c
	var _ helper.Viewer = c
	var _ helper.HealthProber = c
	var _ helper.ReadinessProber = c
	var _ helper.FileDownloader = c
	var _ helper.StreamViewer = c
}

// --- Download mapping ---

func TestDownloadMapsFieldsCorrectly(t *testing.T) {
	t.Parallel()

	var capturedReq *pb.DownloadRequest
	fake := &fieldCapturingPBClient{
		onDownload: func(ctx context.Context, in *pb.DownloadRequest, opts ...gogrpc.CallOption) (*pb.DownloadResponse, error) {
			capturedReq = in
			return &pb.DownloadResponse{Code: 0, Status: "ok", Message: "success", Data: "https://url"}, nil
		},
	}
	c := NewWithServiceClient(fake)
	req := &helper.DownloadRequest{
		Object:     "file.pdf",
		Path:       "docs",
		Bucket:     "hubdev",
		Expires:    120,
		UserID:     10,
		MerchantID: 20,
	}
	res, err := c.Download(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify field mapping to pb
	if capturedReq.Object != "file.pdf" {
		t.Errorf("Object=%q want=file.pdf", capturedReq.Object)
	}
	if capturedReq.Path != "docs" {
		t.Errorf("Path=%q want=docs", capturedReq.Path)
	}
	if capturedReq.Bucket != "hubdev" {
		t.Errorf("Bucket=%q want=hubdev", capturedReq.Bucket)
	}
	if capturedReq.Expires != 120 {
		t.Errorf("Expires=%d want=120", capturedReq.Expires)
	}
	if capturedReq.UserID != 10 {
		t.Errorf("UserID=%d want=10", capturedReq.UserID)
	}
	if capturedReq.MerchantID != 20 {
		t.Errorf("MerchantID=%d want=20", capturedReq.MerchantID)
	}

	// Verify response mapping
	if res.Code != 0 {
		t.Errorf("Code=%d want=0", res.Code)
	}
	if res.Data != "https://url" {
		t.Errorf("Data=%q want=https://url", res.Data)
	}
}

func TestDownloadNilPBResponse(t *testing.T) {
	t.Parallel()

	fake := &fieldCapturingPBClient{
		onDownload: func(ctx context.Context, in *pb.DownloadRequest, opts ...gogrpc.CallOption) (*pb.DownloadResponse, error) {
			return nil, nil
		},
	}
	c := NewWithServiceClient(fake)
	_, err := c.Download(context.Background(), &helper.DownloadRequest{})
	if err == nil {
		t.Fatal("expected error for nil pb response")
	}
}

// --- GenerateViewURL mapping ---

func TestGenerateViewURLMapsFieldsCorrectly(t *testing.T) {
	t.Parallel()

	var capturedReq *pb.DownloadRequest
	fake := &fieldCapturingPBClient{
		onViewUrl: func(ctx context.Context, in *pb.DownloadRequest, opts ...gogrpc.CallOption) (*pb.DownloadResponse, error) {
			capturedReq = in
			return &pb.DownloadResponse{Code: 0, Data: "https://view-url"}, nil
		},
	}
	c := NewWithServiceClient(fake)
	res, err := c.GenerateViewURL(context.Background(), &helper.DownloadRequest{
		Object: "logo.png",
		Bucket: "hubdev",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedReq.Object != "logo.png" {
		t.Errorf("Object=%q want=logo.png", capturedReq.Object)
	}
	if res.Data != "https://view-url" {
		t.Errorf("Data=%q want=https://view-url", res.Data)
	}
}

func TestGenerateViewURLNilPBResponse(t *testing.T) {
	t.Parallel()

	fake := &fieldCapturingPBClient{
		onViewUrl: func(ctx context.Context, in *pb.DownloadRequest, opts ...gogrpc.CallOption) (*pb.DownloadResponse, error) {
			return nil, nil
		},
	}
	c := NewWithServiceClient(fake)
	_, err := c.GenerateViewURL(context.Background(), &helper.DownloadRequest{})
	if err == nil {
		t.Fatal("expected error for nil pb response")
	}
}

// --- Upload mapping ---

func TestUploadMapsFieldsAndParsesData(t *testing.T) {
	t.Parallel()

	stream := &fakeUploadStream{
		res: &pb.UploadResponse{
			Code:    0,
			Status:  "ok",
			Message: "uploaded",
			Data: &pb.UploadData{
				Bucket:       "hubdev",
				Path:         "docs",
				Filename:     "report.pdf",
				URL:          "https://url",
				PresignedURL: "https://presigned",
			},
		},
	}

	c := NewWithServiceClient(&fakePBClient{uploadStream: stream})
	req := &helper.UploadRequest{
		Filename:    "report.pdf",
		Size:        2048,
		ContentType: "application/pdf",
		Content:     []byte("pdf-bytes"),
		Bucket:      "hubdev",
		Path:        "docs",
		Expires:     3600,
		UserID:      1,
		MerchantID:  2,
	}
	res, err := c.Upload(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify sent request to stream
	if stream.req.Filename != "report.pdf" {
		t.Errorf("sent Filename=%q want=report.pdf", stream.req.Filename)
	}
	if stream.req.Size != 2048 {
		t.Errorf("sent Size=%d want=2048", stream.req.Size)
	}

	// Verify response mapping
	if res.Code != 0 {
		t.Errorf("Code=%d want=0", res.Code)
	}
	if res.Data == nil {
		t.Fatal("Data is nil")
	}
	if res.Data.URL != "https://url" {
		t.Errorf("URL=%q want=https://url", res.Data.URL)
	}
	if res.Data.PresignedURL != "https://presigned" {
		t.Errorf("PresignedURL=%q want=https://presigned", res.Data.PresignedURL)
	}
	if res.Data.Filename != "report.pdf" {
		t.Errorf("Filename=%q want=report.pdf", res.Data.Filename)
	}
}

func TestUploadNilPBData(t *testing.T) {
	t.Parallel()

	stream := &fakeUploadStream{
		res: &pb.UploadResponse{
			Code: 0,
			Data: nil,
		},
	}
	c := NewWithServiceClient(&fakePBClient{uploadStream: stream})
	res, err := c.Upload(context.Background(), &helper.UploadRequest{Filename: "a.txt"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Data != nil {
		t.Fatalf("expected nil Data when pb response data is nil")
	}
}

func TestUploadNilPBResponse(t *testing.T) {
	t.Parallel()

	stream := &fakeUploadStream{
		res: nil,
	}
	c := NewWithServiceClient(&fakePBClient{uploadStream: stream})
	_, err := c.Upload(context.Background(), &helper.UploadRequest{Filename: "a.txt"})
	if err == nil {
		t.Fatal("expected error for nil pb upload response")
	}
}

// --- DownloadFile and View explicit stub errors ---

func TestDownloadFileReturnsExplicitStubError(t *testing.T) {
	t.Parallel()

	c := NewWithServiceClient(&fakePBClient{})
	_, err := c.DownloadFile(context.Background(), &helper.DownloadRequest{Object: "a.pdf"})
	if err == nil {
		t.Fatal("expected error from DownloadFile stub")
	}
	if err.Error() != "download_file is not exposed over s3minio grpc yet" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

func TestViewReturnsExplicitStubError(t *testing.T) {
	t.Parallel()

	c := NewWithServiceClient(&fakePBClient{})
	_, err := c.View(context.Background(), &helper.ViewRequest{Path: "/img.png"})
	if err == nil {
		t.Fatal("expected error from View stub")
	}
	if err.Error() != "view stream is not exposed over s3minio grpc yet" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

// --- Health/Ready normalization ---

func TestHealthNormalizesStatusToLowercase(t *testing.T) {
	t.Parallel()

	c := NewWithServiceClient(&fakePBClient{
		healthRes: &pb.HealthResponse{Status: "OK"},
	})
	res, err := c.Health(context.Background(), &helper.HealthRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "ok" {
		t.Fatalf("Status=%q want=ok (lowercase)", res.Status)
	}
	if res.Code != 0 {
		t.Fatalf("Code=%d want=0 for ok status", res.Code)
	}
}

func TestHealthUnavailableStatus(t *testing.T) {
	t.Parallel()

	c := NewWithServiceClient(&fakePBClient{
		healthRes: &pb.HealthResponse{Status: "degraded"},
	})
	res, err := c.Health(context.Background(), &helper.HealthRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Code == 0 {
		t.Fatalf("expected non-zero code for non-ok status")
	}
}

func TestReadyDerivedFromHealth(t *testing.T) {
	t.Parallel()

	c := NewWithServiceClient(&fakePBClient{
		healthRes: &pb.HealthResponse{Status: "ok"},
	})
	res, err := c.Ready(context.Background(), &helper.ReadyRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "ok" {
		t.Fatalf("Status=%q want=ok", res.Status)
	}
	if res.Dependencies == nil || res.Dependencies["grpc"] != "ok" {
		t.Fatalf("Dependencies[grpc]=%q want=ok", res.Dependencies["grpc"])
	}
}

func TestReadyNilHealthFallback(t *testing.T) {
	t.Parallel()

	c := NewWithServiceClient(&fakePBClient{
		healthRes: &pb.HealthResponse{Status: ""},
	})
	res, err := c.Ready(context.Background(), &helper.ReadyRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "unavailable" {
		t.Fatalf("Status=%q want=unavailable when health status empty", res.Status)
	}
}

// --- Error propagation across all operations ---

func TestAllOperationsPropagatePBErrors(t *testing.T) {
	t.Parallel()

	transportErr := errors.New("transport unavailable")
	c := NewWithServiceClient(&fakePBClient{err: transportErr})

	t.Run("Download", func(t *testing.T) {
		t.Parallel()
		_, err := c.Download(context.Background(), &helper.DownloadRequest{})
		if !errors.Is(err, transportErr) {
			t.Fatalf("expected transport error, got: %v", err)
		}
	})

	t.Run("GenerateViewURL", func(t *testing.T) {
		t.Parallel()
		_, err := c.GenerateViewURL(context.Background(), &helper.DownloadRequest{})
		if !errors.Is(err, transportErr) {
			t.Fatalf("expected transport error, got: %v", err)
		}
	})

	t.Run("Upload", func(t *testing.T) {
		t.Parallel()
		_, err := c.Upload(context.Background(), &helper.UploadRequest{})
		if !errors.Is(err, transportErr) {
			t.Fatalf("expected transport error, got: %v", err)
		}
	})

	t.Run("Health", func(t *testing.T) {
		t.Parallel()
		_, err := c.Health(context.Background(), &helper.HealthRequest{})
		if !errors.Is(err, transportErr) {
			t.Fatalf("expected transport error, got: %v", err)
		}
	})

	t.Run("Ready", func(t *testing.T) {
		t.Parallel()
		_, err := c.Ready(context.Background(), &helper.ReadyRequest{})
		if !errors.Is(err, transportErr) {
			t.Fatalf("expected transport error, got: %v", err)
		}
	})
}

// --- Constructor tests ---

func TestNewWithServiceClientNotNil(t *testing.T) {
	t.Parallel()

	c := NewWithServiceClient(&fakePBClient{})
	if c == nil {
		t.Fatal("NewWithServiceClient returned nil")
	}
}

// fieldCapturingPBClient captures the request for field verification.
type fieldCapturingPBClient struct {
	onDownload func(ctx context.Context, in *pb.DownloadRequest, opts ...gogrpc.CallOption) (*pb.DownloadResponse, error)
	onViewUrl  func(ctx context.Context, in *pb.DownloadRequest, opts ...gogrpc.CallOption) (*pb.DownloadResponse, error)
}

func (f *fieldCapturingPBClient) Upload(ctx context.Context, opts ...gogrpc.CallOption) (pb.S3MinIOService_UploadClient, error) {
	return nil, errors.New("not implemented")
}

func (f *fieldCapturingPBClient) Download(ctx context.Context, in *pb.DownloadRequest, opts ...gogrpc.CallOption) (*pb.DownloadResponse, error) {
	if f.onDownload != nil {
		return f.onDownload(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (f *fieldCapturingPBClient) GeneratePresignedUrl(ctx context.Context, in *pb.DownloadRequest, opts ...gogrpc.CallOption) (*pb.DownloadResponse, error) {
	return nil, errors.New("not implemented")
}

func (f *fieldCapturingPBClient) GenerateViewUrl(ctx context.Context, in *pb.DownloadRequest, opts ...gogrpc.CallOption) (*pb.DownloadResponse, error) {
	if f.onViewUrl != nil {
		return f.onViewUrl(ctx, in, opts...)
	}
	return nil, errors.New("not implemented")
}

func (f *fieldCapturingPBClient) Health(ctx context.Context, in *pb.HealthRequest, opts ...gogrpc.CallOption) (*pb.HealthResponse, error) {
	return nil, errors.New("not implemented")
}
