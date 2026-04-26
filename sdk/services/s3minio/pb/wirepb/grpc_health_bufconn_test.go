package pb

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type healthStub struct {
	UnimplementedS3MinIOServiceServer
}

func (healthStub) Health(ctx context.Context, in *HealthRequest) (*HealthResponse, error) {
	_ = ctx
	_ = in
	return &HealthResponse{Status: "SERVING"}, nil
}

func TestS3MinIOService_GRPC_HealthViaBufConn(t *testing.T) {
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	RegisterS3MinIOServiceServer(srv, healthStub{})
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("Serve: %v", err)
		}
	}()
	defer srv.Stop()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("DialContext: %v", err)
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
