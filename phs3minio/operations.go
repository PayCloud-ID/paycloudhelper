package phs3minio

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
)

var (
	ErrNilDownloadResponse = errors.New("s3minio download response is nil")
	ErrNilUploadResponse   = errors.New("s3minio upload response is nil")
	ErrNilUploadData       = errors.New("s3minio upload response data is nil")
)

// GetPresignedURL builds and executes a download request then validates the result.
func GetPresignedURL(ctx context.Context, d Downloader, object string, userID, merchantID int64, args ...interface{}) (string, error) {
	req := BuildDownloadRequest(object, userID, merchantID, args...)
	res, err := d.Download(ctx, &req)
	if err != nil {
		return "", err
	}
	if res == nil {
		return "", ErrNilDownloadResponse
	}
	if res.Code != CodeOK {
		msg := res.Message
		if msg == "" {
			msg = "download failed"
		}
		return "", fmt.Errorf("s3minio download: %s", msg)
	}
	if res.Data == "" {
		return object, nil
	}
	return res.Data, nil
}

// UploadByMultipart builds and executes an upload request from multipart data.
func UploadByMultipart(ctx context.Context, u Uploader, userID, merchantID int64, path string, file multipart.File, fileHeader *multipart.FileHeader, args ...interface{}) (*UploadResult, error) {
	req, err := BuildUploadRequestForMultipart(userID, merchantID, path, file, fileHeader, args...)
	if err != nil {
		return nil, err
	}
	return UploadByRequest(ctx, u, &req)
}

// UploadByFile builds and executes an upload request from file path.
func UploadByFile(ctx context.Context, u Uploader, userID, merchantID int64, path, fileLocation string, args ...interface{}) (*UploadResult, error) {
	req, err := BuildUploadRequestForFile(userID, merchantID, path, fileLocation, args...)
	if err != nil {
		return nil, err
	}
	return UploadByRequest(ctx, u, &req)
}

// UploadByRequest executes an upload request and validates the result.
func UploadByRequest(ctx context.Context, u Uploader, req *UploadRequest) (*UploadResult, error) {
	res, err := u.Upload(ctx, req)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrNilUploadResponse
	}
	if res.Code != CodeOK {
		msg := res.Message
		if msg == "" {
			msg = "upload failed"
		}
		return nil, fmt.Errorf("s3minio upload: %s", msg)
	}
	if res.Data == nil {
		return nil, ErrNilUploadData
	}
	return res.Data, nil
}
