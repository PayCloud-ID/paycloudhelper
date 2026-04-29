// Package pb exposes the service-scoped S3MinIO protobuf compatibility surface.
package pb

import (
	"context"

	wirepb "github.com/PayCloud-ID/paycloudhelper/sdk/services/s3minio/pb/wirepb"
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
	wire wirepb.S3MinIOServiceClient
}

func NewS3MinIOServiceClient(cc grpc.ClientConnInterface) S3MinIOServiceClient {
	return &s3MinIOServiceClient{wire: wirepb.NewS3MinIOServiceClient(cc)}
}

func (c *s3MinIOServiceClient) Upload(ctx context.Context, opts ...grpc.CallOption) (S3MinIOService_UploadClient, error) {
	stream, err := c.wire.Upload(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return &s3MinIOServiceUploadClient{S3MinIOService_UploadClient: stream}, nil
}

func (c *s3MinIOServiceClient) Download(ctx context.Context, in *DownloadRequest, opts ...grpc.CallOption) (*DownloadResponse, error) {
	out, err := c.wire.Download(ctx, &wirepb.DownloadRequest{
		Object:     in.Object,
		Bucket:     in.Bucket,
		Path:       in.Path,
		Expires:    in.Expires,
		UserId:     in.UserID,
		MerchantId: in.MerchantID,
	}, opts...)
	if err != nil {
		return nil, err
	}

	return &DownloadResponse{
		Code:    out.Code,
		Status:  out.Status,
		Message: out.Message,
		Data:    out.Data,
	}, nil
}

func (c *s3MinIOServiceClient) GeneratePresignedUrl(ctx context.Context, in *DownloadRequest, opts ...grpc.CallOption) (*DownloadResponse, error) {
	out, err := c.wire.GeneratePresignedUrl(ctx, &wirepb.DownloadRequest{
		Object:     in.Object,
		Bucket:     in.Bucket,
		Path:       in.Path,
		Expires:    in.Expires,
		UserId:     in.UserID,
		MerchantId: in.MerchantID,
	}, opts...)
	if err != nil {
		return nil, err
	}

	return &DownloadResponse{
		Code:    out.Code,
		Status:  out.Status,
		Message: out.Message,
		Data:    out.Data,
	}, nil
}

func (c *s3MinIOServiceClient) GenerateViewUrl(ctx context.Context, in *DownloadRequest, opts ...grpc.CallOption) (*DownloadResponse, error) {
	out, err := c.wire.GenerateViewUrl(ctx, &wirepb.DownloadRequest{
		Object:     in.Object,
		Bucket:     in.Bucket,
		Path:       in.Path,
		Expires:    in.Expires,
		UserId:     in.UserID,
		MerchantId: in.MerchantID,
	}, opts...)
	if err != nil {
		return nil, err
	}

	return &DownloadResponse{
		Code:    out.Code,
		Status:  out.Status,
		Message: out.Message,
		Data:    out.Data,
	}, nil
}

func (c *s3MinIOServiceClient) Health(ctx context.Context, in *HealthRequest, opts ...grpc.CallOption) (*HealthResponse, error) {
	_ = in
	out, err := c.wire.Health(ctx, &wirepb.HealthRequest{}, opts...)
	if err != nil {
		return nil, err
	}

	return &HealthResponse{Status: out.Status}, nil
}

type S3MinIOService_UploadClient interface {
	Send(*UploadRequest) error
	CloseAndRecv() (*UploadResponse, error)
	grpc.ClientStream
}

type s3MinIOServiceUploadClient struct {
	wirepb.S3MinIOService_UploadClient
}

func (x *s3MinIOServiceUploadClient) Send(m *UploadRequest) error {
	return x.S3MinIOService_UploadClient.Send(&wirepb.UploadRequest{
		Filename:    m.Filename,
		Size:        m.Size,
		ContentType: m.ContentType,
		Content:     m.Content,
		Bucket:      m.Bucket,
		Path:        m.Path,
		Expires:     m.Expires,
		UserId:      m.UserID,
		MerchantId:  m.MerchantID,
	})
}

func (x *s3MinIOServiceUploadClient) CloseAndRecv() (*UploadResponse, error) {
	m, err := x.S3MinIOService_UploadClient.CloseAndRecv()
	if err != nil {
		return nil, err
	}

	out := &UploadResponse{
		Code:    m.Code,
		Status:  m.Status,
		Message: m.Message,
	}
	if m.Data != nil {
		out.Data = &UploadData{
			Bucket:       m.Data.Bucket,
			Path:         m.Data.Path,
			Filename:     m.Data.Filename,
			URL:          m.Data.Url,
			PresignedURL: m.Data.PresignedUrl,
		}
	}

	return out, nil
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
