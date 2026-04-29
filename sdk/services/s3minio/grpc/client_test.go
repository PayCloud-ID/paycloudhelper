package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/PayCloud-ID/paycloudhelper/sdk/services/s3minio/helper"
	"github.com/PayCloud-ID/paycloudhelper/sdk/services/s3minio/pb"
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
