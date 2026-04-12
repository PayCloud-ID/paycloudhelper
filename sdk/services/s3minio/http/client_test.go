package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"bitbucket.org/paycloudid/paycloudhelper/sdk/services/s3minio/helper"
)

func TestGenerateViewURL(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"status":"OK","message":"ok","data":"https://view"}`))
	}))
	defer ts.Close()

	c := New(ts.URL, nil)
	res, err := c.GenerateViewURL(context.Background(), &helper.DownloadRequest{Object: "a"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Data != "https://view" {
		t.Fatalf("data=%q want=%q", res.Data, "https://view")
	}
}

func TestHealthReady(t *testing.T) {
	t.Parallel()

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"code":200,"status":"OK","message":"ready"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":200,"status":"OK","message":"healthy"}`))
	}))
	defer ts.Close()

	c := New(ts.URL, nil)
	health, err := c.Health(context.Background(), &helper.HealthRequest{})
	if err != nil {
		t.Fatalf("unexpected health error: %v", err)
	}
	if health.Status != "ok" {
		t.Fatalf("health status=%q want ok", health.Status)
	}

	ready, err := c.Ready(context.Background(), &helper.ReadyRequest{})
	if err != nil {
		t.Fatalf("unexpected ready error: %v", err)
	}
	if ready.Status != "ok" {
		t.Fatalf("ready status=%q want ok", ready.Status)
	}
}

func TestClientCompatibilitySurface(t *testing.T) {
	t.Parallel()

	var _ helper.Client = (*Client)(nil)
}
