// Package pb exposes the service-scoped S3MinIO protobuf compatibility surface.
package pb

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

const ServiceName = "/s3minio.S3MinIOService"

type UploadRequest struct {
	Filename    string
	Size        uint64
	ContentType string
	Content     []byte
	Bucket      string
	Path        string
	Expires     uint32
	UserID      int64
	MerchantID  int64
}

type UploadData struct {
	Bucket       string
	Path         string
	Filename     string
	URL          string
	PresignedURL string
}

type UploadResponse struct {
	Code    uint32
	Status  string
	Message string
	Data    *UploadData
}

type DownloadRequest struct {
	Object     string
	Bucket     string
	Path       string
	Expires    int32
	UserID     int64
	MerchantID int64
}

type DownloadResponse struct {
	Code    uint32
	Status  string
	Message string
	Data    string
}

type HealthRequest struct{}

type HealthResponse struct {
	Status string
}

type S3MinIOServiceClient interface {
	Upload(ctx context.Context, opts ...grpc.CallOption) (S3MinIOService_UploadClient, error)
	Download(ctx context.Context, in *DownloadRequest, opts ...grpc.CallOption) (*DownloadResponse, error)
	GeneratePresignedUrl(ctx context.Context, in *DownloadRequest, opts ...grpc.CallOption) (*DownloadResponse, error)
	GenerateViewUrl(ctx context.Context, in *DownloadRequest, opts ...grpc.CallOption) (*DownloadResponse, error)
	Health(ctx context.Context, in *HealthRequest, opts ...grpc.CallOption) (*HealthResponse, error)
}

type s3MinIOServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewS3MinIOServiceClient(cc grpc.ClientConnInterface) S3MinIOServiceClient {
	return &s3MinIOServiceClient{cc: cc}
}

func (c *s3MinIOServiceClient) Upload(ctx context.Context, opts ...grpc.CallOption) (S3MinIOService_UploadClient, error) {
	stream, err := c.cc.NewStream(ctx, &S3MinIOService_ServiceDesc.Streams[0], ServiceName+"/Upload", opts...)
	if err != nil {
		return nil, err
	}
	return &s3MinIOServiceUploadClient{ClientStream: stream}, nil
}

func (c *s3MinIOServiceClient) Download(ctx context.Context, in *DownloadRequest, opts ...grpc.CallOption) (*DownloadResponse, error) {
	out := new(DownloadResponse)
	err := c.cc.Invoke(ctx, ServiceName+"/Download", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *s3MinIOServiceClient) GeneratePresignedUrl(ctx context.Context, in *DownloadRequest, opts ...grpc.CallOption) (*DownloadResponse, error) {
	out := new(DownloadResponse)
	err := c.cc.Invoke(ctx, ServiceName+"/GeneratePresignedUrl", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *s3MinIOServiceClient) GenerateViewUrl(ctx context.Context, in *DownloadRequest, opts ...grpc.CallOption) (*DownloadResponse, error) {
	out := new(DownloadResponse)
	err := c.cc.Invoke(ctx, ServiceName+"/GenerateViewUrl", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *s3MinIOServiceClient) Health(ctx context.Context, in *HealthRequest, opts ...grpc.CallOption) (*HealthResponse, error) {
	out := new(HealthResponse)
	err := c.cc.Invoke(ctx, ServiceName+"/Health", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

type S3MinIOService_UploadClient interface {
	Send(*UploadRequest) error
	CloseAndRecv() (*UploadResponse, error)
	grpc.ClientStream
}

type s3MinIOServiceUploadClient struct {
	grpc.ClientStream
}

func (x *s3MinIOServiceUploadClient) Send(m *UploadRequest) error {
	return x.ClientStream.SendMsg(m)
}

func (x *s3MinIOServiceUploadClient) CloseAndRecv() (*UploadResponse, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(UploadResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var S3MinIOService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "s3minio.S3MinIOService",
	Methods: []grpc.MethodDesc{
		{MethodName: "Download"},
		{MethodName: "GeneratePresignedUrl"},
		{MethodName: "GenerateViewUrl"},
		{MethodName: "Health"},
	},
	Streams: []grpc.StreamDesc{
		{StreamName: "Upload", ClientStreams: true},
	},
	Metadata: "s3minio.proto",
}

func OKCode() uint32 {
	return uint32(codes.OK)
}
