package pb

import (
	"context"
	"io"
	"net"
	"testing"

	wirepb "github.com/PayCloud-ID/paycloudhelper/sdk/services/s3minio/pb/wirepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
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

func (s *testS3MinioServer) Upload(stream wirepb.S3MinIOService_UploadServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return stream.SendAndClose(&wirepb.UploadResponse{
		Code:    200,
		Status:  "success",
		Message: "ok",
		Data: &wirepb.UploadData{
			Bucket:       "buck",
			Path:         "objects",
			Filename:     "f.bin",
			Url:          "https://example/u",
			PresignedUrl: "https://example/ps",
		},
	})
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

	preRes, err := client.GeneratePresignedUrl(context.Background(), &DownloadRequest{
		Object: "o", Bucket: "b", Path: "p",
	})
	if err != nil {
		t.Fatalf("presign failed: %v", err)
	}
	if preRes == nil || preRes.Data != "p" {
		t.Fatalf("unexpected presign response: %#v", preRes)
	}

	hres, err := client.Health(context.Background(), &HealthRequest{})
	if err != nil || hres == nil || hres.Status != "ok" {
		t.Fatalf("health: err=%v hres=%#v", err, hres)
	}

	up, err := client.Upload(context.Background())
	if err != nil {
		t.Fatalf("upload stream: %v", err)
	}
	if err := up.Send(&UploadRequest{
		Filename: "f.bin", Size: 3, ContentType: "application/octet-stream",
		Content: []byte{1, 2, 3}, Bucket: "buck", Path: "objects",
		UserID: 1, MerchantID: 2,
	}); err != nil {
		t.Fatalf("send: %v", err)
	}
	ures, err := up.CloseAndRecv()
	if err != nil {
		t.Fatalf("close recv: %v", err)
	}
	if ures == nil || ures.Data == nil || ures.Data.Bucket != "buck" || ures.Data.PresignedURL != "https://example/ps" {
		t.Fatalf("unexpected upload response: %#v", ures)
	}
}

// errDownloadServer implements Download only; other RPCs stay unimplemented.
type errDownloadServer struct {
	wirepb.UnimplementedS3MinIOServiceServer
}

func (errDownloadServer) Download(context.Context, *wirepb.DownloadRequest) (*wirepb.DownloadResponse, error) {
	return nil, status.Error(codes.Unavailable, "broker down")
}

func TestPbClientDownloadPropagatesGrpcError(t *testing.T) {
	listener := bufconn.Listen(testBufSize)
	server := grpc.NewServer()
	wirepb.RegisterS3MinIOServiceServer(server, errDownloadServer{})

	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := NewS3MinIOServiceClient(conn)
	_, err = client.Download(context.Background(), &DownloadRequest{Object: "x"})
	if err == nil {
		t.Fatal("expected error from Download")
	}
	if _, ok := status.FromError(err); !ok {
		t.Fatalf("want grpc status error, got %T %v", err, err)
	}
}

type errPresignServer struct {
	wirepb.UnimplementedS3MinIOServiceServer
}

func (errPresignServer) GeneratePresignedUrl(context.Context, *wirepb.DownloadRequest) (*wirepb.DownloadResponse, error) {
	return nil, status.Error(codes.Internal, "presign failed")
}

type errHealthServer struct {
	wirepb.UnimplementedS3MinIOServiceServer
}

func (errHealthServer) Health(context.Context, *wirepb.HealthRequest) (*wirepb.HealthResponse, error) {
	return nil, status.Error(codes.Unavailable, "no health")
}

func TestPbClientHealthPropagatesGrpcError(t *testing.T) {
	listener := bufconn.Listen(testBufSize)
	server := grpc.NewServer()
	wirepb.RegisterS3MinIOServiceServer(server, errHealthServer{})

	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := NewS3MinIOServiceClient(conn)
	_, err = client.Health(context.Background(), &HealthRequest{})
	if err == nil {
		t.Fatal("expected error from Health")
	}
	if _, ok := status.FromError(err); !ok {
		t.Fatalf("want grpc status error, got %T %v", err, err)
	}
}

func TestPbClientGeneratePresignedUrlPropagatesGrpcError(t *testing.T) {
	listener := bufconn.Listen(testBufSize)
	server := grpc.NewServer()
	wirepb.RegisterS3MinIOServiceServer(server, errPresignServer{})

	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	client := NewS3MinIOServiceClient(conn)
	_, err = client.GeneratePresignedUrl(context.Background(), &DownloadRequest{Object: "x"})
	if err == nil {
		t.Fatal("expected error from GeneratePresignedUrl")
	}
	if _, ok := status.FromError(err); !ok {
		t.Fatalf("want grpc status error, got %T %v", err, err)
	}
}
