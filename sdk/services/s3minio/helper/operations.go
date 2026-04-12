package helper

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
	ErrNilHealthResponse   = errors.New("s3minio health response is nil")
	ErrNilReadyResponse    = errors.New("s3minio ready response is nil")
	ErrNilFileResponse     = errors.New("s3minio download file response is nil")
)

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

func UploadByMultipart(ctx context.Context, u Uploader, userID, merchantID int64, path string, file multipart.File, fileHeader *multipart.FileHeader, args ...interface{}) (*UploadResult, error) {
	req, err := BuildUploadRequestForMultipart(userID, merchantID, path, file, fileHeader, args...)
	if err != nil {
		return nil, err
	}
	return UploadByRequest(ctx, u, &req)
}

func UploadByFile(ctx context.Context, u Uploader, userID, merchantID int64, path, fileLocation string, args ...interface{}) (*UploadResult, error) {
	req, err := BuildUploadRequestForFile(userID, merchantID, path, fileLocation, args...)
	if err != nil {
		return nil, err
	}
	return UploadByRequest(ctx, u, &req)
}

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

func GetViewURL(ctx context.Context, v Viewer, object string, userID, merchantID int64, args ...interface{}) (string, error) {
	req := BuildDownloadRequest(object, userID, merchantID, args...)
	res, err := v.GenerateViewURL(ctx, &req)
	if err != nil {
		return "", err
	}
	if res == nil {
		return "", ErrNilDownloadResponse
	}
	if res.Code != CodeOK {
		msg := res.Message
		if msg == "" {
			msg = "generate view url failed"
		}
		return "", fmt.Errorf("s3minio view: %s", msg)
	}
	if res.Data == "" {
		return object, nil
	}
	return res.Data, nil
}

func CheckHealth(ctx context.Context, p HealthProber) (*HealthResponse, error) {
	res, err := p.Health(ctx, &HealthRequest{})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrNilHealthResponse
	}
	if res.Code != 0 && res.Code != CodeOK {
		msg := res.Message
		if msg == "" {
			msg = "health check failed"
		}
		return nil, fmt.Errorf("s3minio health: %s", msg)
	}
	return res, nil
}

func CheckReady(ctx context.Context, p ReadinessProber) (*ReadyResponse, error) {
	res, err := p.Ready(ctx, &ReadyRequest{})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrNilReadyResponse
	}
	if res.Code != 0 && res.Code != CodeOK {
		msg := res.Message
		if msg == "" {
			msg = "readiness check failed"
		}
		return nil, fmt.Errorf("s3minio ready: %s", msg)
	}
	return res, nil
}

func DownloadFileByRequest(ctx context.Context, d FileDownloader, req *DownloadRequest) (*FileDownloadResponse, error) {
	res, err := d.DownloadFile(ctx, req)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, ErrNilFileResponse
	}
	if res.Code != 0 && res.Code != CodeOK {
		msg := res.Message
		if msg == "" {
			msg = "download file failed"
		}
		return nil, fmt.Errorf("s3minio download file: %s", msg)
	}
	return res, nil
}
