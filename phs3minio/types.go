// Package phs3minio provides transport-neutral DTOs and helpers for S3MinIO gRPC
// clients. Callers map these types to their repo-specific protobuf messages.
package phs3minio

import "context"

// CodeOK is the gRPC status code for success (google.golang.org/grpc/codes.OK == 0).
const CodeOK uint32 = 0

// DownloadRequest mirrors common s3minio DownloadRequest fields used by PayCloud services.
type DownloadRequest struct {
	Object     string
	Path       string
	Bucket     string
	Expires    int32
	UserID     int64
	MerchantID int64
}

// DownloadResponse mirrors a unary Download RPC result (code/message/data).
type DownloadResponse struct {
	Code    uint32
	Message string
	Data    string
}

// UploadRequest mirrors common s3minio UploadRequest fields.
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

// UploadResult is the payload returned on successful upload.
type UploadResult struct {
	Filename     string
	URL          string
	PresignedURL string
}

// UploadResponse mirrors an Upload RPC result (code/message + optional data).
type UploadResponse struct {
	Code    uint32
	Message string
	Data    *UploadResult
}

// Downloader abstracts the download/presigned operation.
type Downloader interface {
	Download(ctx context.Context, req *DownloadRequest) (*DownloadResponse, error)
}

// Uploader abstracts the upload operation.
type Uploader interface {
	Upload(ctx context.Context, req *UploadRequest) (*UploadResponse, error)
}

// Client is kept as a backward-compatible alias for tests and adapters.
type Client interface {
	Downloader
	Uploader
}
