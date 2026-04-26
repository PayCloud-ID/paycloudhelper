package helper

import (
	"context"
	"errors"
	"mime/multipart"
	"os"
	"path/filepath"
	"testing"
)

type fakeDownloader struct {
	res *DownloadResponse
	err error
}

func (f fakeDownloader) Download(_ context.Context, _ *DownloadRequest) (*DownloadResponse, error) {
	return f.res, f.err
}

type fakeUploader struct {
	res *UploadResponse
	err error
}

func (f fakeUploader) Upload(_ context.Context, _ *UploadRequest) (*UploadResponse, error) {
	return f.res, f.err
}

func TestGetPresignedURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		d       Downloader
		want    string
		wantErr bool
	}{
		{
			name: "success",
			d:    fakeDownloader{res: &DownloadResponse{Code: CodeOK, Data: "https://signed"}},
			want: "https://signed",
		},
		{
			name:    "download error",
			d:       fakeDownloader{err: errors.New("dial failed")},
			wantErr: true,
		},
		{
			name:    "nil response",
			d:       fakeDownloader{},
			wantErr: true,
		},
		{
			name:    "non ok response",
			d:       fakeDownloader{res: &DownloadResponse{Code: 13, Message: "boom"}},
			wantErr: true,
		},
		{
			name:    "non ok empty message uses default",
			d:       fakeDownloader{res: &DownloadResponse{Code: 500, Message: ""}},
			wantErr: true,
		},
		{
			name: "empty data fallback",
			d:    fakeDownloader{res: &DownloadResponse{Code: CodeOK}},
			want: "obj.txt",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := GetPresignedURL(context.Background(), tt.d, "obj.txt", 1, 2, "path", "bucket", 60)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("GetPresignedURL() got=%q want=%q", got, tt.want)
			}
		})
	}
}

func TestUploadByRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		u       Uploader
		wantErr bool
	}{
		{
			name: "success",
			u:    fakeUploader{res: &UploadResponse{Code: CodeOK, Data: &UploadResult{URL: "https://file"}}},
		},
		{
			name:    "upload error",
			u:       fakeUploader{err: errors.New("send failed")},
			wantErr: true,
		},
		{
			name:    "nil response",
			u:       fakeUploader{},
			wantErr: true,
		},
		{
			name:    "response code error",
			u:       fakeUploader{res: &UploadResponse{Code: 2, Message: "bad"}},
			wantErr: true,
		},
		{
			name:    "nil data",
			u:       fakeUploader{res: &UploadResponse{Code: CodeOK, Data: nil}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := UploadByRequest(context.Background(), tt.u, &UploadRequest{Filename: "a.txt"})
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestUploadByMultipart(t *testing.T) {
	t.Parallel()

	content := "hello"
	f, err := os.CreateTemp(t.TempDir(), "upload-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp() error: %v", err)
	}
	defer f.Close()

	if _, err = f.WriteString(content); err != nil {
		t.Fatalf("WriteString() error: %v", err)
	}
	if _, err = f.Seek(0, 0); err != nil {
		t.Fatalf("Seek() error: %v", err)
	}

	header := &multipart.FileHeader{
		Filename: "x.txt",
		Size:     int64(len(content)),
		Header:   map[string][]string{"Content-Type": {"text/plain"}},
	}

	res, err := UploadByMultipart(
		context.Background(),
		fakeUploader{res: &UploadResponse{Code: CodeOK, Data: &UploadResult{URL: "ok"}}},
		1,
		2,
		"docs",
		multipart.File(f),
		header,
		"bucket",
		30,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res == nil || res.URL != "ok" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestUploadByFile_success(t *testing.T) {
	t.Parallel()
	p := filepath.Join(t.TempDir(), "up.bin")
	if err := os.WriteFile(p, []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	u := fakeUploader{res: &UploadResponse{Code: CodeOK, Data: &UploadResult{URL: "https://u"}}}
	out, err := UploadByFile(context.Background(), u, 1, 2, "p", p)
	if err != nil {
		t.Fatal(err)
	}
	if out == nil || out.URL != "https://u" {
		t.Fatalf("got %+v", out)
	}
}

func TestUploadByFile_buildError(t *testing.T) {
	t.Parallel()
	_, err := UploadByFile(context.Background(), fakeUploader{}, 1, 2, "p", filepath.Join(t.TempDir(), "missing-upload.bin"))
	if err == nil {
		t.Fatal("expected error from missing file")
	}
}
