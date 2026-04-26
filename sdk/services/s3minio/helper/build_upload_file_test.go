package helper

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildUploadRequestForFile(t *testing.T) {
	dir := t.TempDir()
	loc := filepath.Join(dir, "blob.bin")
	if err := os.WriteFile(loc, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	req, err := BuildUploadRequestForFile(1, 2, "obj/p", loc, "my-bucket", 3600)
	if err != nil {
		t.Fatal(err)
	}
	if req.Bucket != "my-bucket" || req.Expires != 3600 {
		t.Fatalf("bucket=%q expires=%d", req.Bucket, req.Expires)
	}
	if string(req.Content) != "hello" || req.Size != 5 {
		t.Fatalf("content=%q size=%d", req.Content, req.Size)
	}
}

func TestBuildUploadRequestForFile_openError(t *testing.T) {
	_, err := BuildUploadRequestForFile(1, 2, "p", filepath.Join(t.TempDir(), "missing-no-such-file"))
	if err == nil {
		t.Fatal("expected error")
	}
}
