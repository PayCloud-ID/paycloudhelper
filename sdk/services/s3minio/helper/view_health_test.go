package helper

import (
	"context"
	"errors"
	"testing"
)

type fakeViewer struct {
	res *DownloadResponse
	err error
}

func (f fakeViewer) GenerateViewURL(_ context.Context, _ *DownloadRequest) (*DownloadResponse, error) {
	return f.res, f.err
}

type fakeHealthProber struct {
	res *HealthResponse
	err error
}

func (f fakeHealthProber) Health(_ context.Context, _ *HealthRequest) (*HealthResponse, error) {
	return f.res, f.err
}

type fakeReadinessProber struct {
	res *ReadyResponse
	err error
}

func (f fakeReadinessProber) Ready(_ context.Context, _ *ReadyRequest) (*ReadyResponse, error) {
	return f.res, f.err
}

type fakeFileDownloader struct {
	res *FileDownloadResponse
	err error
}

func (f fakeFileDownloader) DownloadFile(_ context.Context, _ *DownloadRequest) (*FileDownloadResponse, error) {
	return f.res, f.err
}

func TestGetViewURL(t *testing.T) {
	t.Parallel()

	viewer := fakeViewer{res: &DownloadResponse{Code: CodeOK, Data: "https://view"}}
	got, err := GetViewURL(context.Background(), viewer, "obj.png", 1, 2, "ktp", "hubdev", 60)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://view" {
		t.Fatalf("GetViewURL() got=%q want=%q", got, "https://view")
	}
}

func TestGetViewURLError(t *testing.T) {
	t.Parallel()

	_, err := GetViewURL(context.Background(), fakeViewer{err: errors.New("boom")}, "obj.png", 1, 2)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetViewURL_nonOKUsesDefaultMessageWhenEmpty(t *testing.T) {
	t.Parallel()
	_, err := GetViewURL(context.Background(), fakeViewer{res: &DownloadResponse{Code: 500, Message: ""}}, "obj.png", 1, 2)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCheckHealth(t *testing.T) {
	t.Parallel()

	res, err := CheckHealth(context.Background(), fakeHealthProber{res: &HealthResponse{Code: CodeOK, Status: "ok"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "ok" {
		t.Fatalf("CheckHealth() status=%q want=%q", res.Status, "ok")
	}
}

func TestCheckHealthError(t *testing.T) {
	t.Parallel()

	_, err := CheckHealth(context.Background(), fakeHealthProber{res: &HealthResponse{Code: 13, Message: "down"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCheckReady(t *testing.T) {
	t.Parallel()

	res, err := CheckReady(context.Background(), fakeReadinessProber{res: &ReadyResponse{Code: CodeOK, Status: "ready"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != "ready" {
		t.Fatalf("CheckReady() status=%q want=%q", res.Status, "ready")
	}
}

func TestCheckReadyError(t *testing.T) {
	t.Parallel()

	_, err := CheckReady(context.Background(), fakeReadinessProber{res: &ReadyResponse{Code: 14, Message: "not ready"}})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestDownloadFileByRequest(t *testing.T) {
	t.Parallel()

	res, err := DownloadFileByRequest(context.Background(), fakeFileDownloader{res: &FileDownloadResponse{Code: CodeOK, Filename: "a.pdf"}}, &DownloadRequest{Object: "a.pdf"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Filename != "a.pdf" {
		t.Fatalf("DownloadFileByRequest() filename=%q want=%q", res.Filename, "a.pdf")
	}
}

func TestDownloadFileByRequestError(t *testing.T) {
	t.Parallel()

	_, err := DownloadFileByRequest(context.Background(), fakeFileDownloader{err: errors.New("io fail")}, &DownloadRequest{Object: "a.pdf"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
