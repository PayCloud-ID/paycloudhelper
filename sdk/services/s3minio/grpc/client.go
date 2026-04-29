// Package grpc exposes the service-scoped S3MinIO gRPC transport adapter.
package grpc

import (
	extgrpc "google.golang.org/grpc"

	"github.com/PayCloud-ID/paycloudhelper/sdk/services/s3minio/pb"
)

type Client struct {
	pb pb.S3MinIOServiceClient
}

func NewWithServiceClient(client pb.S3MinIOServiceClient) *Client {
	return &Client{pb: client}
}

func NewWithConn(conn extgrpc.ClientConnInterface) *Client {
	return &Client{pb: pb.NewS3MinIOServiceClient(conn)}
}
