package grpc

import (
	"context"
	"errors"
	"testing"

	"bitbucket.org/paycloudid/paycloudhelper/sdk/services/s3minio/helper"
	"bitbucket.org/paycloudid/paycloudhelper/sdk/services/s3minio/pb"
	gogrpc "google.golang.org/grpc"
)

// fakeUploadStream implements pb.S3MinIOService_UploadClient.
type fakeUploadStream struct {
	req *pb.UploadRequest
	res *pb.UploadResponse
	err error
	gogrpc.ClientStream
}

// Send records the request and returns the stream error.
func (f *fakeUploadStream) Send(r *pb.UploadRequest) error {
	f.req = r
	return f.err
}

// CloseAndRecv closes the stream and returns the response.
func (f *fakeUploadStream) CloseAndRecv() (*pb.UploadResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.res, nil
}

// fakePBClient implements pb.S3MinIOServiceClient.
type fakePBClient struct {
	err          error
	uploadStream *fakeUploadStream
	downloadRes  *pb.DownloadResponse
	viewRes      *pb.DownloadResponse
	healthRes    *pb.HealthResponse
}

// Upload returns a fake upload stream.
func (f *fakePBClient) Upload(context.Context, ...gogrpc.CallOption) (pb.S3MinIOService_UploadClient, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.uploadStream, nil
}

// Download returns a fake download response.
func (f *fakePBClient) Download(context.Context, *pb.DownloadRequest, ...gogrpc.CallOption) (*pb.DownloadResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.downloadRes, nil
}

// GeneratePresignedUrl returns a fake presigned URL response.
func (f *fakePBClient) GeneratePresignedUrl(context.Context, *pb.DownloadRequest, ...gogrpc.CallOption) (*pb.DownloadResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.downloadRes, nil
}

// GenerateViewUrl returns a fake view URL response.
func (f *fakePBClient) GenerateViewUrl(context.Context, *pb.DownloadRequest, ...gogrpc.CallOption) (*pb.DownloadResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.viewRes, nil
}

// Health returns a fake health response.
func (f *fakePBClient) Health(context.Context, *pb.HealthRequest, ...gogrpc.CallOption) (*pb.HealthResponse, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.healthRes, nil
}

// TestClientImplementsInterfaces verifies the client implements expected interfaces.
func TestClientImplementsInterfaces(t *testing.T) {
	t.Parallel()

	c := NewWithServiceClient(&fakePBClient{})

	var _ helper.Uploader = c
	var _ helper.Downloader = c
	var _ helper.Viewer = c
	var _ helper.HealthProber = c
}

// TestDownloadSuccess verifies successful download operation.
func TestDownloadSuccess(t *testing.T) {
	t.Parallel()

	c := NewWithServiceClient(&fakePBClient{
		downloadRes: &pb.DownloadResponse{
			Code: pb.OKCode(),
			Data: "ok",
		},
	})

	res, err := c.Download(context.Background(), &helper.DownloadRequest{Object: "a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Data != "ok" {
		t.Fatalf("data=%q want=ok", res.Data)
	}
}

// TestUploadSuccess verifies successful upload operation.
func TestUploadSuccess(t *testing.T) {
	t.Parallel()

	stream := &fakeUploadStream{
		res: &pb.UploadResponse{
			Code: pb.OKCode(),
			Data: &pb.UploadData{
				URL:          "u",
				PresignedURL: "p",
			},
		},
	}

	c := NewWithServiceClient(&fakePBClient{uploadStream: stream})

	res, err := c.Upload(context.Background(), &helper.UploadRequest{Filename: "a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Data == nil || res.Data.URL != "u" {
		t.Fatalf("unexpected upload result: %#v", res.Data)
	}
}

// TestHealthReady verifies health and readiness checks.
func TestHealthReady(t *testing.T) {
	t.Parallel()

	c := NewWithServiceClient(&fakePBClient{
		healthRes: &pb.HealthResponse{Status: "ok"},
	})

	health, err := c.Health(context.Background(), &helper.HealthRequest{})
	if err != nil {
		t.Fatalf("unexpected health error: %v", err)
	}
	if health.Status != "ok" {
		t.Fatalf("status=%q want ok", health.Status)
	}

	ready, err := c.Ready(context.Background(), &helper.ReadyRequest{})
	if err != nil {
		t.Fatalf("unexpected ready error: %v", err)
	}
	if ready.Dependencies["grpc"] != "ok" {
		t.Fatalf("dependency grpc=%q want ok", ready.Dependencies["grpc"])
	}
}

// TestClientErrors verifies error propagation.
func TestClientErrors(t *testing.T) {
	t.Parallel()

	c := NewWithServiceClient(&fakePBClient{err: errors.New("down")})

	if _, err := c.Download(context.Background(), &helper.DownloadRequest{}); err == nil {
		t.Fatal("expected download error")
	}
	if _, err := c.Upload(context.Background(), &helper.UploadRequest{}); err == nil {
		t.Fatal("expected upload error")
	}
	if _, err := c.Health(context.Background(), &helper.HealthRequest{}); err == nil {
		t.Fatal("expected health error")
	}
}

func TestDownload_nilResponse(t *testing.T) {
	t.Parallel()
	c := NewWithServiceClient(&fakePBClient{downloadRes: nil})
	_, err := c.Download(context.Background(), &helper.DownloadRequest{Object: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestGenerateViewURL_nilResponse(t *testing.T) {
	t.Parallel()
	c := NewWithServiceClient(&fakePBClient{viewRes: nil})
	_, err := c.GenerateViewURL(context.Background(), &helper.DownloadRequest{Object: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpload_nilCloseAndRecvResponse(t *testing.T) {
	t.Parallel()
	stream := &fakeUploadStream{res: nil, err: nil}
	c := NewWithServiceClient(&fakePBClient{uploadStream: stream})
	_, err := c.Upload(context.Background(), &helper.UploadRequest{Filename: "a"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestHealth_nonOKStatus(t *testing.T) {
	t.Parallel()
	c := NewWithServiceClient(&fakePBClient{
		healthRes: &pb.HealthResponse{Status: "DOWN"},
	})
	health, err := c.Health(context.Background(), &helper.HealthRequest{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if health.Status != "DOWN" {
		t.Fatalf("status=%q", health.Status)
	}
	if health.Code == 200 {
		t.Fatalf("expected non-200 code for bad status, got %d", health.Code)
	}
}

func TestReady_emptyHealthStatusBecomesUnavailable(t *testing.T) {
	t.Parallel()
	c := NewWithServiceClient(&fakePBClient{
		healthRes: &pb.HealthResponse{Status: ""},
	})
	ready, err := c.Ready(context.Background(), &helper.ReadyRequest{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if ready.Status != "unavailable" {
		t.Fatalf("ready.Status=%q want unavailable", ready.Status)
	}
}

type stubReadiness struct {
	res *helper.ReadyResponse
	err error
}

func (s stubReadiness) Ready(ctx context.Context, _ *helper.ReadyRequest) (*helper.ReadyResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.res, nil
}

// TestCheckReadyDelegates exercises the grpc package wrapper that forwards to helper.CheckReady.
func TestCheckReadyDelegates(t *testing.T) {
	t.Parallel()
	res, err := CheckReady(context.Background(), stubReadiness{
		res: &helper.ReadyResponse{Code: helper.CodeOK, Status: "ready"},
	})
	if err != nil {
		t.Fatalf("CheckReady: %v", err)
	}
	if res == nil || res.Status != "ready" {
		t.Fatalf("unexpected response: %#v", res)
	}
}

type stubHealth struct {
	res *helper.HealthResponse
	err error
}

func (s stubHealth) Health(ctx context.Context, _ *helper.HealthRequest) (*helper.HealthResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.res, nil
}

func TestCheckHealthDelegates(t *testing.T) {
	t.Parallel()
	res, err := CheckHealth(context.Background(), stubHealth{
		res: &helper.HealthResponse{Code: helper.CodeOK, Status: "ok"},
	})
	if err != nil {
		t.Fatalf("CheckHealth: %v", err)
	}
	if res == nil || res.Status != "ok" {
		t.Fatalf("unexpected response: %#v", res)
	}
}
