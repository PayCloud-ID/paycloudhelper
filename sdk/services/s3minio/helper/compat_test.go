package helper

import (
	"context"
	"testing"
)

func TestCompatibilitySurface(t *testing.T) {
	t.Parallel()

	var _ Downloader = compatFakeDownloader{}
	var _ Uploader = compatFakeUploader{}
}

type compatFakeDownloader struct{}

func (compatFakeDownloader) Download(_ context.Context, _ *DownloadRequest) (*DownloadResponse, error) {
	return nil, nil
}

type compatFakeUploader struct{}

func (compatFakeUploader) Upload(_ context.Context, _ *UploadRequest) (*UploadResponse, error) {
	return nil, nil
}
