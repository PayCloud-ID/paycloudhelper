// Package facade provides stable constructors for service-scoped S3MinIO SDK usage.
package facade

import (
	nethttp "net/http"

	sdkgrpc "github.com/PayCloud-ID/paycloudhelper/sdk/services/s3minio/grpc"
	sdkhttp "github.com/PayCloud-ID/paycloudhelper/sdk/services/s3minio/http"
	extgrpc "google.golang.org/grpc"
)

func NewGRPC(conn extgrpc.ClientConnInterface) *sdkgrpc.Client {
	return sdkgrpc.NewWithConn(conn)
}

func NewHTTPBridge(baseURL string, client *nethttp.Client) *sdkhttp.Client {
	return sdkhttp.New(baseURL, client)
}
