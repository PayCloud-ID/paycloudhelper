package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PayCloud-ID/paycloudhelper/sdk/services/s3minio/helper"
)

func TestNew_usesDefaultHTTPClientWhenNil(t *testing.T) {
	if c := New("http://example.invalid", nil); c == nil {
		t.Fatal("nil client")
	}
}

func TestHealth_OK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Errorf("path=%s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	c := New(ts.URL, ts.Client())
	out, err := c.Health(context.Background(), &helper.HealthRequest{})
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if out.Status != "ok" {
		t.Fatalf("status=%q", out.Status)
	}
}

func TestGenerateViewURL_success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/generate_view_url" {
			t.Errorf("path=%s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":200,"status":"OK","message":"ok","data":"\"signed-url\""}`))
	}))
	defer ts.Close()
	c := New(ts.URL, ts.Client())
	out, err := c.GenerateViewURL(context.Background(), &helper.DownloadRequest{Object: "o"})
	if err != nil {
		t.Fatalf("GenerateViewURL: %v", err)
	}
	if out.Data == "" {
		t.Fatalf("empty data: %+v", out)
	}
}

func TestView_success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RequestURI(), "/api/v2/view?path=") {
			t.Errorf("uri=%s", r.URL.RequestURI())
		}
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("view-body"))
	}))
	defer ts.Close()
	c := New(ts.URL, ts.Client())
	out, err := c.View(context.Background(), &helper.ViewRequest{Path: "docs/a"})
	if err != nil {
		t.Fatalf("View: %v", err)
	}
	if string(out.Data) != "view-body" {
		t.Fatalf("data=%q", out.Data)
	}
}

func TestDownloadFile_nonOK(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	c := New(ts.URL, ts.Client())
	_, err := c.DownloadFile(context.Background(), &helper.DownloadRequest{Object: "o", Bucket: "b"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDownload_invalidJSONBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer ts.Close()
	c := New(ts.URL, ts.Client())
	_, err := c.Download(context.Background(), &helper.DownloadRequest{Object: "x"})
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestUpload_success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/upload" {
			t.Errorf("path=%s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":200,"status":"OK","message":"done","data":{"filename":"f.txt","url":"http://u","presigned_url":"http://p"}}`))
	}))
	defer ts.Close()
	c := New(ts.URL, ts.Client())
	out, err := c.Upload(context.Background(), &helper.UploadRequest{
		Filename: "f.txt",
		Content:  []byte("hello"),
		Bucket:   "b",
		Path:     "p",
		Expires:  3600,
	})
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if out.Data == nil || out.Data.URL == "" {
		t.Fatalf("unexpected data: %+v", out.Data)
	}
}

func TestHealth_nonOKStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()
	c := New(ts.URL, ts.Client())
	_, err := c.Health(context.Background(), &helper.HealthRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReady_nonOKStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()
	c := New(ts.URL, ts.Client())
	out, err := c.Ready(context.Background(), &helper.ReadyRequest{})
	if err != nil {
		t.Fatalf("Ready: %v", err)
	}
	if out.Status != "unavailable" {
		t.Fatalf("status=%q", out.Status)
	}
}

func TestDownload_jsonErrorFromServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":400,"status":"ERR","message":"bad","data":null}`))
	}))
	defer ts.Close()
	c := New(ts.URL, ts.Client())
	_, err := c.Download(context.Background(), &helper.DownloadRequest{Object: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestView_nilRequest(t *testing.T) {
	c := New("http://localhost", http.DefaultClient)
	_, err := c.View(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestView_emptyPath(t *testing.T) {
	c := New("http://localhost", http.DefaultClient)
	_, err := c.View(context.Background(), &helper.ViewRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpload_nilRequest(t *testing.T) {
	c := New("http://localhost", http.DefaultClient)
	_, err := c.Upload(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDownloadFile_success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("file-bytes"))
	}))
	defer ts.Close()
	c := New(ts.URL, ts.Client())
	out, err := c.DownloadFile(context.Background(), &helper.DownloadRequest{Object: "o", Bucket: "b", Path: "p"})
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	if string(out.Data) != "file-bytes" {
		t.Fatalf("data=%q", out.Data)
	}
}

func TestView_nonOKStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer ts.Close()
	c := New(ts.URL, ts.Client())
	_, err := c.View(context.Background(), &helper.ViewRequest{Path: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestUpload_nonOKStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	defer ts.Close()
	c := New(ts.URL, ts.Client())
	_, err := c.Upload(context.Background(), &helper.UploadRequest{
		Filename: "f.txt", Content: []byte("a"),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

// Covers postDownloadLike HTTP error branch when JSON envelope message is empty (uses res.Status).
func TestGenerateViewURL_nonOKUsesResponseStatusWhenMessageEmpty(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"code":503,"status":"ERR","message":"","data":null}`))
	}))
	defer ts.Close()
	c := New(ts.URL, ts.Client())
	_, err := c.GenerateViewURL(context.Background(), &helper.DownloadRequest{Object: "o"})
	if err == nil {
		t.Fatal("expected error")
	}
}
