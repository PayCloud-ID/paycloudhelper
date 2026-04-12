package grpc

import (
	"context"
	"errors"

	"bitbucket.org/paycloudid/paycloudhelper/sdk/services/s3minio/helper"
	"bitbucket.org/paycloudid/paycloudhelper/sdk/services/s3minio/pb"
)

var _ helper.Client = (*Client)(nil)

func (c *Client) Download(ctx context.Context, req *helper.DownloadRequest) (*helper.DownloadResponse, error) {
	res, err := c.pb.Download(ctx, &pb.DownloadRequest{
		Object:     req.Object,
		Path:       req.Path,
		Bucket:     req.Bucket,
		Expires:    req.Expires,
		UserID:     req.UserID,
		MerchantID: req.MerchantID,
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errors.New("minio download response is nil")
	}

	return &helper.DownloadResponse{Code: res.Code, Message: res.Message, Data: res.Data}, nil
}

func (c *Client) GenerateViewURL(ctx context.Context, req *helper.DownloadRequest) (*helper.DownloadResponse, error) {
	res, err := c.pb.GenerateViewUrl(ctx, &pb.DownloadRequest{
		Object:     req.Object,
		Path:       req.Path,
		Bucket:     req.Bucket,
		Expires:    req.Expires,
		UserID:     req.UserID,
		MerchantID: req.MerchantID,
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errors.New("minio view response is nil")
	}

	return &helper.DownloadResponse{Code: res.Code, Message: res.Message, Data: res.Data}, nil
}

func (c *Client) Upload(ctx context.Context, req *helper.UploadRequest) (*helper.UploadResponse, error) {
	stream, err := c.pb.Upload(ctx)
	if err != nil {
		return nil, err
	}

	err = stream.Send(&pb.UploadRequest{
		Filename:    req.Filename,
		Size:        req.Size,
		ContentType: req.ContentType,
		Content:     req.Content,
		Path:        req.Path,
		Bucket:      req.Bucket,
		Expires:     req.Expires,
		UserID:      req.UserID,
		MerchantID:  req.MerchantID,
	})
	if err != nil {
		return nil, err
	}

	res, err := stream.CloseAndRecv()
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, errors.New("minio upload response is nil")
	}

	var data *helper.UploadResult
	if res.Data != nil {
		data = &helper.UploadResult{
			Filename:     res.Data.Filename,
			URL:          res.Data.URL,
			PresignedURL: res.Data.PresignedURL,
		}
	}

	return &helper.UploadResponse{Code: res.Code, Message: res.Message, Data: data}, nil
}

func (c *Client) DownloadFile(_ context.Context, _ *helper.DownloadRequest) (*helper.FileDownloadResponse, error) {
	return nil, errors.New("download_file is not exposed over s3minio grpc yet")
}

func (c *Client) View(_ context.Context, _ *helper.ViewRequest) (*helper.ViewResponse, error) {
	return nil, errors.New("view stream is not exposed over s3minio grpc yet")
}
