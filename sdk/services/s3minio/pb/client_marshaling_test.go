package pb

import (
	"context"
	"net"
	"testing"

	wirepb "github.com/PayCloud-ID/paycloudhelper/sdk/services/s3minio/pb/wirepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const testBufSize = 1024 * 1024

type testS3MinioServer struct {
	wirepb.UnimplementedS3MinIOServiceServer
}

func (s *testS3MinioServer) Download(_ context.Context, in *wirepb.DownloadRequest) (*wirepb.DownloadResponse, error) {
	return &wirepb.DownloadResponse{
		Code:    200,
		Status:  "success",
		Message: "ok",
		Data:    in.Object,
	}, nil
}

func (s *testS3MinioServer) GeneratePresignedUrl(_ context.Context, in *wirepb.DownloadRequest) (*wirepb.DownloadResponse, error) {
	return &wirepb.DownloadResponse{
		Code:    200,
		Status:  "success",
		Message: "ok",
		Data:    in.Path,
	}, nil
}

func (s *testS3MinioServer) GenerateViewUrl(_ context.Context, in *wirepb.DownloadRequest) (*wirepb.DownloadResponse, error) {
	return &wirepb.DownloadResponse{
		Code:    200,
		Status:  "success",
		Message: "ok",
		Data:    in.Bucket,
	}, nil
}

func (s *testS3MinioServer) Health(_ context.Context, _ *wirepb.HealthRequest) (*wirepb.HealthResponse, error) {
	return &wirepb.HealthResponse{Status: "ok"}, nil
}

func TestClientUsesWireProtoMessages(t *testing.T) {
	listener := bufconn.Listen(testBufSize)
	server := grpc.NewServer()
	wirepb.RegisterS3MinIOServiceServer(server, &testS3MinioServer{})

	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	dialer := func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer),
	)
	if err != nil {
		t.Fatalf("failed to dial bufnet: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	client := NewS3MinIOServiceClient(conn)

	downloadRes, err := client.Download(context.Background(), &DownloadRequest{
		Object:     "avatar.png",
		Bucket:     "hubdev",
		Path:       "profile",
		Expires:    120,
		UserID:     1,
		MerchantID: 2,
	})
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if downloadRes == nil || downloadRes.Data != "avatar.png" {
		t.Fatalf("unexpected download response: %#v", downloadRes)
	}

	viewRes, err := client.GenerateViewUrl(context.Background(), &DownloadRequest{
		Object: "avatar.png",
		Bucket: "hubstaging",
		Path:   "profile",
	})
	if err != nil {
		t.Fatalf("generate view failed: %v", err)
	}
	if viewRes == nil || viewRes.Data != "hubstaging" {
		t.Fatalf("unexpected view response: %#v", viewRes)
	}
}
