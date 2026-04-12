package helper

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// ============================================================================
// Acceptance Criteria Validation Tests
// These tests validate that paycloudhelper meets all plan acceptance criteria
// for the S3MinIO integration consolidation (plan-s3minioIntegrationConsolidationV1).
// ============================================================================

// --- Criterion 1: Helper exposes full provider capability parity ---

func TestAllRequiredInterfacesExist(t *testing.T) {
	t.Parallel()

	// Verify all 7 interfaces are defined and their method signatures match the plan.
	var d Downloader
	var u Uploader
	var v Viewer
	var hp HealthProber
	var rp ReadinessProber
	var fd FileDownloader
	var sv StreamViewer

	// Suppress unused variable warnings while ensuring compile-time check.
	_ = d
	_ = u
	_ = v
	_ = hp
	_ = rp
	_ = fd
	_ = sv
}

func TestClientInterfaceComposesAllCapabilities(t *testing.T) {
	t.Parallel()

	// The Client interface must embed all 7 capability interfaces.
	clientType := reflect.TypeOf((*Client)(nil)).Elem()

	expected := []string{
		"Download",
		"Upload",
		"GenerateViewURL",
		"Health",
		"Ready",
		"DownloadFile",
		"View",
	}

	for _, method := range expected {
		m, ok := clientType.MethodByName(method)
		if !ok {
			t.Errorf("Client interface missing method: %s", method)
			continue
		}
		// Every method should accept context.Context as first non-receiver arg
		if m.Type.NumIn() < 2 {
			t.Errorf("Client.%s should have at least 2 inputs (context + request)", method)
		}
	}
}

// --- Criterion 2: Supports view-url-first flows ---

func TestGetViewURLSupportsOptionalArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []interface{}
		want string
	}{
		{
			name: "with path, bucket, expires",
			args: []interface{}{"ktp", "hubdev", 60},
			want: "https://view",
		},
		{
			name: "with path only",
			args: []interface{}{"ktp"},
			want: "https://view",
		},
		{
			name: "no optional args",
			args: nil,
			want: "https://view",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			viewer := fakeViewer{res: &DownloadResponse{Code: CodeOK, Data: "https://view"}}
			got, err := GetViewURL(context.Background(), viewer, "obj.png", 1, 2, tt.args...)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("GetViewURL() got=%q want=%q", got, tt.want)
			}
		})
	}
}

// --- Criterion 3: Health and readiness abstractions ---

func TestHealthResponseHasRequiredFields(t *testing.T) {
	t.Parallel()

	hr := HealthResponse{
		Code:    0,
		Message: "ok",
		Status:  "ok",
	}
	if hr.Code != CodeOK {
		t.Fatalf("HealthResponse.Code=%d, want 0", hr.Code)
	}
}

func TestReadyResponseHasDependenciesMap(t *testing.T) {
	t.Parallel()

	rr := ReadyResponse{
		Code:         0,
		Message:      "ready",
		Status:       "ok",
		Dependencies: map[string]string{"grpc": "ok", "minio": "ok"},
	}
	if len(rr.Dependencies) != 2 {
		t.Fatalf("ReadyResponse.Dependencies length=%d, want 2", len(rr.Dependencies))
	}
}

// --- Criterion 12: Service-scoped SDK layout established ---

func TestServiceScopedLayoutExists(t *testing.T) {
	t.Parallel()

	// Find the repo root: we are in sdk/services/s3minio/helper/
	_, thisFile, _, _ := runtime.Caller(0)
	s3minioDir := filepath.Dir(filepath.Dir(thisFile))

	requiredDirs := []string{
		"helper",
		"grpc",
		"http",
		"pb",
		"proto",
		"facade",
	}

	for _, dir := range requiredDirs {
		path := filepath.Join(s3minioDir, dir)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("required directory missing: %s (error: %v)", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s exists but is not a directory", dir)
		}
	}
}

func TestProtoSnapshotExists(t *testing.T) {
	t.Parallel()

	_, thisFile, _, _ := runtime.Caller(0)
	protoFile := filepath.Join(filepath.Dir(filepath.Dir(thisFile)), "proto", "s3minio.proto")

	_, err := os.Stat(protoFile)
	if err != nil {
		t.Fatalf("proto snapshot not found: %s (error: %v)", protoFile, err)
	}
}

// --- DTO completeness ---

func TestAllDTOsHaveRequiredFields(t *testing.T) {
	t.Parallel()

	t.Run("DownloadRequest", func(t *testing.T) {
		t.Parallel()
		dr := DownloadRequest{
			Object:     "file.pdf",
			Path:       "docs",
			Bucket:     "hubdev",
			Expires:    60,
			UserID:     1,
			MerchantID: 2,
		}
		if dr.Object == "" || dr.Bucket == "" {
			t.Fatal("DownloadRequest fields not set correctly")
		}
	})

	t.Run("UploadRequest", func(t *testing.T) {
		t.Parallel()
		ur := UploadRequest{
			Filename:    "test.jpg",
			Size:        1024,
			ContentType: "image/jpeg",
			Content:     []byte("data"),
			Bucket:      "hubdev",
			Path:        "images",
			Expires:     3600,
			UserID:      1,
			MerchantID:  2,
		}
		if ur.Filename == "" || ur.Size == 0 {
			t.Fatal("UploadRequest fields not set correctly")
		}
	})

	t.Run("HealthRequest", func(t *testing.T) {
		t.Parallel()
		_ = HealthRequest{} // Value type, no fields required
	})

	t.Run("HealthResponse", func(t *testing.T) {
		t.Parallel()
		hr := HealthResponse{Code: 0, Message: "ok", Status: "ok"}
		if hr.Status == "" {
			t.Fatal("HealthResponse.Status not set")
		}
	})

	t.Run("ReadyRequest", func(t *testing.T) {
		t.Parallel()
		_ = ReadyRequest{}
	})

	t.Run("ReadyResponse", func(t *testing.T) {
		t.Parallel()
		rr := ReadyResponse{
			Code:         0,
			Message:      "ready",
			Status:       "ok",
			Dependencies: map[string]string{"minio": "ok"},
		}
		if rr.Dependencies == nil {
			t.Fatal("ReadyResponse.Dependencies is nil")
		}
	})

	t.Run("FileDownloadResponse", func(t *testing.T) {
		t.Parallel()
		fdr := FileDownloadResponse{
			Code:        200,
			Message:     "ok",
			Status:      "200 OK",
			Data:        []byte("pdf-bytes"),
			ContentType: "application/pdf",
			Filename:    "report.pdf",
		}
		if fdr.ContentType == "" || fdr.Filename == "" {
			t.Fatal("FileDownloadResponse missing fields")
		}
	})

	t.Run("ViewRequest", func(t *testing.T) {
		t.Parallel()
		vr := ViewRequest{Path: "images/logo.png"}
		if vr.Path == "" {
			t.Fatal("ViewRequest.Path not set")
		}
	})

	t.Run("ViewResponse", func(t *testing.T) {
		t.Parallel()
		vr := ViewResponse{
			Code:        200,
			Message:     "ok",
			Status:      "200 OK",
			Data:        []byte("png-bytes"),
			ContentType: "image/png",
		}
		if vr.ContentType == "" {
			t.Fatal("ViewResponse.ContentType not set")
		}
	})
}

// --- Builder function coverage ---

func TestBuildDownloadRequestAllArgs(t *testing.T) {
	t.Parallel()

	req := BuildDownloadRequest("file.pdf", 1, 2, "docs", "hubdev", 120)
	if req.Object != "file.pdf" {
		t.Fatalf("Object=%q want=file.pdf", req.Object)
	}
	if req.Path != "docs" {
		t.Fatalf("Path=%q want=docs", req.Path)
	}
	if req.Bucket != "hubdev" {
		t.Fatalf("Bucket=%q want=hubdev", req.Bucket)
	}
	if req.Expires != 120 {
		t.Fatalf("Expires=%d want=120", req.Expires)
	}
}

func TestBuildDownloadRequestMinimalArgs(t *testing.T) {
	t.Parallel()

	req := BuildDownloadRequest("file.pdf", 1, 2)
	if req.Object != "file.pdf" {
		t.Fatalf("Object=%q want=file.pdf", req.Object)
	}
	if req.Path != "" {
		t.Fatalf("Path=%q want empty", req.Path)
	}
	if req.Bucket != "" {
		t.Fatalf("Bucket=%q want empty", req.Bucket)
	}
}

func TestBuildDownloadRequestIgnoresInvalidArgTypes(t *testing.T) {
	t.Parallel()

	// Passing wrong types for optional args should not panic
	req := BuildDownloadRequest("file.pdf", 1, 2, 42, true, "wrong position")
	if req.Object != "file.pdf" {
		t.Fatalf("Object=%q want=file.pdf", req.Object)
	}
	// Invalid types should result in defaults
	if req.Path != "" {
		t.Fatalf("Path should be empty for non-string arg, got=%q", req.Path)
	}
}

func TestBuildHealthAndReadyRequests(t *testing.T) {
	t.Parallel()

	h := BuildHealthRequest()
	r := BuildReadyRequest()
	_ = h
	_ = r
}

func TestBuildViewRequest(t *testing.T) {
	t.Parallel()

	vr := BuildViewRequest("images/logo.png")
	if vr.Path != "images/logo.png" {
		t.Fatalf("ViewRequest.Path=%q want=images/logo.png", vr.Path)
	}
}

// --- Error sentinel coverage ---

func TestSentinelErrors(t *testing.T) {
	t.Parallel()

	sentinels := []error{
		ErrNilDownloadResponse,
		ErrNilUploadResponse,
		ErrNilUploadData,
		ErrNilHealthResponse,
		ErrNilReadyResponse,
		ErrNilFileResponse,
	}

	for _, s := range sentinels {
		if s == nil {
			t.Fatalf("sentinel error is nil")
		}
		if s.Error() == "" {
			t.Fatalf("sentinel error has empty message")
		}
	}
}

// --- Operation edge case coverage ---

func TestCheckHealthNilResponse(t *testing.T) {
	t.Parallel()

	_, err := CheckHealth(context.Background(), fakeHealthProber{res: nil})
	if err == nil {
		t.Fatal("expected error for nil health response")
	}
	if err != ErrNilHealthResponse {
		t.Fatalf("expected ErrNilHealthResponse, got: %v", err)
	}
}

func TestCheckReadyNilResponse(t *testing.T) {
	t.Parallel()

	_, err := CheckReady(context.Background(), fakeReadinessProber{res: nil})
	if err == nil {
		t.Fatal("expected error for nil ready response")
	}
	if err != ErrNilReadyResponse {
		t.Fatalf("expected ErrNilReadyResponse, got: %v", err)
	}
}

func TestGetPresignedURLNilResponse(t *testing.T) {
	t.Parallel()

	_, err := GetPresignedURL(context.Background(), fakeDownloader{res: nil}, "obj.txt", 1, 2)
	if err == nil {
		t.Fatal("expected error for nil response")
	}
	if err != ErrNilDownloadResponse {
		t.Fatalf("expected ErrNilDownloadResponse, got: %v", err)
	}
}

func TestUploadByRequestNilData(t *testing.T) {
	t.Parallel()

	_, err := UploadByRequest(context.Background(),
		fakeUploader{res: &UploadResponse{Code: CodeOK, Data: nil}},
		&UploadRequest{Filename: "a.txt"},
	)
	if err == nil {
		t.Fatal("expected error for nil upload data")
	}
	if err != ErrNilUploadData {
		t.Fatalf("expected ErrNilUploadData, got: %v", err)
	}
}

func TestDownloadFileNilResponse(t *testing.T) {
	t.Parallel()

	_, err := DownloadFileByRequest(context.Background(),
		fakeFileDownloader{res: nil},
		&DownloadRequest{Object: "a.pdf"},
	)
	if err == nil {
		t.Fatal("expected error for nil file response")
	}
	if err != ErrNilFileResponse {
		t.Fatalf("expected ErrNilFileResponse, got: %v", err)
	}
}

func TestGetViewURLNilResponse(t *testing.T) {
	t.Parallel()

	_, err := GetViewURL(context.Background(), fakeViewer{res: nil}, "obj.png", 1, 2)
	if err == nil {
		t.Fatal("expected error for nil response")
	}
	if err != ErrNilDownloadResponse {
		t.Fatalf("expected ErrNilDownloadResponse, got: %v", err)
	}
}

func TestGetViewURLNonZeroCodeReturnsError(t *testing.T) {
	t.Parallel()

	_, err := GetViewURL(context.Background(),
		fakeViewer{res: &DownloadResponse{Code: 13, Message: "permission denied"}},
		"obj.png", 1, 2,
	)
	if err == nil {
		t.Fatal("expected error for non-OK code")
	}
}

func TestGetViewURLEmptyDataFallback(t *testing.T) {
	t.Parallel()

	got, err := GetViewURL(context.Background(),
		fakeViewer{res: &DownloadResponse{Code: CodeOK, Data: ""}},
		"obj.png", 1, 2,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// When data is empty, should fall back to original object
	if got != "obj.png" {
		t.Fatalf("expected fallback to object name, got=%q", got)
	}
}

func TestCheckHealthNonZeroCodeError(t *testing.T) {
	t.Parallel()

	_, err := CheckHealth(context.Background(),
		fakeHealthProber{res: &HealthResponse{Code: 503, Message: ""}},
	)
	if err == nil {
		t.Fatal("expected error for non-OK health code")
	}
}

func TestCheckReadyNonZeroCodeError(t *testing.T) {
	t.Parallel()

	_, err := CheckReady(context.Background(),
		fakeReadinessProber{res: &ReadyResponse{Code: 503, Message: ""}},
	)
	if err == nil {
		t.Fatal("expected error for non-OK ready code")
	}
}

func TestDownloadFileNonZeroCodeError(t *testing.T) {
	t.Parallel()

	_, err := DownloadFileByRequest(context.Background(),
		fakeFileDownloader{res: &FileDownloadResponse{Code: 500, Message: ""}},
		&DownloadRequest{Object: "a.pdf"},
	)
	if err == nil {
		t.Fatal("expected error for non-OK download file code")
	}
}
