// Package helper exposes the service-scoped S3MinIO SDK contracts.
package helper

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
	Status  string
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
	Status  string
	Message string
	Data    *UploadResult
}

// HealthRequest is a transport-neutral liveness probe request.
type HealthRequest struct{}

// HealthResponse represents liveness output from a provider or adapter.
type HealthResponse struct {
	Code    uint32
	Message string
	Status  string
}

// ReadyRequest is a transport-neutral readiness probe request.
type ReadyRequest struct{}

// ReadyResponse represents readiness output and optional dependency map.
type ReadyResponse struct {
	Code         uint32
	Message      string
	Status       string
	Dependencies map[string]string
}

// FileDownloadResponse represents HTTP download-file style payload.
type FileDownloadResponse struct {
	Code        uint32
	Message     string
	Status      string
	Data        []byte
	ContentType string
	Filename    string
}

// ViewRequest represents HTTP stream view request semantics.
type ViewRequest struct {
	Path string
}

// ViewResponse represents HTTP stream view response semantics.
type ViewResponse struct {
	Code        uint32
	Message     string
	Status      string
	Data        []byte
	ContentType string
}

// Downloader abstracts the download/presigned operation.
type Downloader interface {
	Download(ctx context.Context, req *DownloadRequest) (*DownloadResponse, error)
}

// Uploader abstracts the upload operation.
type Uploader interface {
	Upload(ctx context.Context, req *UploadRequest) (*UploadResponse, error)
}

// Viewer abstracts generate-view-url operation.
type Viewer interface {
	GenerateViewURL(ctx context.Context, req *DownloadRequest) (*DownloadResponse, error)
}

// HealthProber abstracts service liveness checks.
type HealthProber interface {
	Health(ctx context.Context, req *HealthRequest) (*HealthResponse, error)
}

// ReadinessProber abstracts dependency-readiness checks.
type ReadinessProber interface {
	Ready(ctx context.Context, req *ReadyRequest) (*ReadyResponse, error)
}

// FileDownloader abstracts download_file behavior.
type FileDownloader interface {
	DownloadFile(ctx context.Context, req *DownloadRequest) (*FileDownloadResponse, error)
}

// StreamViewer abstracts HTTP stream view behavior.
type StreamViewer interface {
	View(ctx context.Context, req *ViewRequest) (*ViewResponse, error)
}

// Client groups all supported S3MinIO helper capabilities.
type Client interface {
	Downloader
	Uploader
	Viewer
	HealthProber
	ReadinessProber
	FileDownloader
	StreamViewer
}
