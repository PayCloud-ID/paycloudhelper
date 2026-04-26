package pb

import (
	"context"
	"io"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// fullS3Stub implements unary RPCs and client-streaming Upload for bufconn tests.
type fullS3Stub struct {
	UnimplementedS3MinIOServiceServer
}

func (fullS3Stub) Download(_ context.Context, _ *DownloadRequest) (*DownloadResponse, error) {
	return &DownloadResponse{Code: 200, Status: "ok", Data: "download-url"}, nil
}

func (fullS3Stub) GeneratePresignedUrl(_ context.Context, _ *DownloadRequest) (*DownloadResponse, error) {
	return &DownloadResponse{Code: 200, Status: "ok", Data: "presigned-url"}, nil
}

func (fullS3Stub) GenerateViewUrl(_ context.Context, _ *DownloadRequest) (*DownloadResponse, error) {
	return &DownloadResponse{Code: 200, Status: "ok", Data: "view-url"}, nil
}

func (fullS3Stub) Upload(stream grpc.ClientStreamingServer[UploadRequest, UploadResponse]) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&UploadResponse{Code: 200, Status: "uploaded"})
		}
		if err != nil {
			return err
		}
	}
}

func dialBufConn(t *testing.T, lis *bufconn.Listener) *grpc.ClientConn {
	t.Helper()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestS3MinIOService_GRPC_UnaryRPCsViaBufConn(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	RegisterS3MinIOServiceServer(srv, fullS3Stub{})
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Serve: %v", err)
		}
	}()
	t.Cleanup(func() { srv.Stop() })

	conn := dialBufConn(t, lis)
	cli := NewS3MinIOServiceClient(conn)
	ctx := context.Background()
	req := &DownloadRequest{Bucket: "b", Path: "p", Object: "f"}

	dl, err := cli.Download(ctx, req)
	if err != nil || dl.GetData() != "download-url" {
		t.Fatalf("Download: err=%v data=%q", err, dl.GetData())
	}
	pre, err := cli.GeneratePresignedUrl(ctx, req)
	if err != nil || pre.GetData() != "presigned-url" {
		t.Fatalf("GeneratePresignedUrl: err=%v data=%q", err, pre.GetData())
	}
	vu, err := cli.GenerateViewUrl(ctx, req)
	if err != nil || vu.GetData() != "view-url" {
		t.Fatalf("GenerateViewUrl: err=%v data=%q", err, vu.GetData())
	}
}

func TestS3MinIOService_GRPC_UploadStreamViaBufConn(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	RegisterS3MinIOServiceServer(srv, fullS3Stub{})
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Serve: %v", err)
		}
	}()
	t.Cleanup(func() { srv.Stop() })

	conn := dialBufConn(t, lis)
	cli := NewS3MinIOServiceClient(conn)
	stream, err := cli.Upload(context.Background())
	if err != nil {
		t.Fatalf("Upload stream: %v", err)
	}
	if err := stream.Send(&UploadRequest{Filename: "a.txt", Content: []byte("hi")}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend: %v", err)
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv: %v", err)
	}
	if resp.GetCode() != 200 {
		t.Fatalf("response code = %d", resp.GetCode())
	}
}

func noopUnaryServerInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	return handler(ctx, req)
}

func countUnaryClientInterceptor(n *int) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		*n++
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func TestS3MinIOService_GRPC_DownloadWithClientChainUnaryInterceptor(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	RegisterS3MinIOServiceServer(srv, fullS3Stub{})
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Serve: %v", err)
		}
	}()
	t.Cleanup(func() { srv.Stop() })

	var calls int
	ctx := context.Background()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(countUnaryClientInterceptor(&calls)),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	cli := NewS3MinIOServiceClient(conn)
	out, err := cli.Download(ctx, &DownloadRequest{Object: "o"})
	if err != nil || out.GetData() != "download-url" {
		t.Fatalf("Download: err=%v data=%q", err, out.GetData())
	}
	if calls < 1 {
		t.Fatalf("expected client unary interceptor invoked, calls=%d", calls)
	}
}

func TestS3MinIOService_GRPC_UnaryWithServerInterceptor(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(noopUnaryServerInterceptor))
	RegisterS3MinIOServiceServer(srv, fullS3Stub{})
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Serve: %v", err)
		}
	}()
	t.Cleanup(func() { srv.Stop() })

	conn := dialBufConn(t, lis)
	cli := NewS3MinIOServiceClient(conn)
	out, err := cli.Download(context.Background(), &DownloadRequest{})
	if err != nil || out.GetData() != "download-url" {
		t.Fatalf("Download with server interceptor: err=%v data=%q", err, out.GetData())
	}
}

func noopUnaryClientInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	return invoker(ctx, method, req, reply, cc, opts...)
}

func TestS3MinIOService_GRPC_HealthWithClientInterceptor(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	RegisterS3MinIOServiceServer(srv, healthStub{})
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Serve: %v", err)
		}
	}()
	t.Cleanup(func() { srv.Stop() })

	ctx := context.Background()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(noopUnaryClientInterceptor),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	cli := NewS3MinIOServiceClient(conn)
	out, err := cli.Health(ctx, &HealthRequest{})
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if out.GetStatus() != "SERVING" {
		t.Fatalf("Health status = %q", out.GetStatus())
	}
}

// errUnaryStub returns gRPC errors on unary Download for client error-path coverage.
type errUnaryStub struct {
	UnimplementedS3MinIOServiceServer
}

func (errUnaryStub) Download(context.Context, *DownloadRequest) (*DownloadResponse, error) {
	return nil, status.Errorf(codes.Unavailable, "backend unavailable")
}

func TestS3MinIOService_GRPC_DownloadReturnsGrpcError(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	RegisterS3MinIOServiceServer(srv, errUnaryStub{})
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Serve: %v", err)
		}
	}()
	t.Cleanup(func() { srv.Stop() })

	conn := dialBufConn(t, lis)
	cli := NewS3MinIOServiceClient(conn)
	_, err := cli.Download(context.Background(), &DownloadRequest{Object: "x"})
	if err == nil {
		t.Fatal("expected gRPC error")
	}
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("code=%v want Unavailable", status.Code(err))
	}
}

func TestS3MinIOService_GRPC_UploadUnimplemented(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	RegisterS3MinIOServiceServer(srv, healthStub{}) // no Upload
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Serve: %v", err)
		}
	}()
	t.Cleanup(func() { srv.Stop() })

	conn := dialBufConn(t, lis)
	cli := NewS3MinIOServiceClient(conn)
	stream, err := cli.Upload(context.Background())
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	_ = stream.CloseSend()
	_, err = stream.CloseAndRecv()
	if err == nil {
		t.Fatal("expected error from unimplemented Upload")
	}
	if status.Code(err) != codes.Unimplemented {
		t.Fatalf("code = %v want Unimplemented", status.Code(err))
	}
}
