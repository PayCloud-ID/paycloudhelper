// Package http exposes the service-scoped S3MinIO HTTP bridge adapter.
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	nethttp "net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/PayCloud-ID/paycloudhelper/sdk/services/s3minio/helper"
)

type Client struct {
	baseURL    string
	httpClient *nethttp.Client
}

type responseEnvelope struct {
	Code    uint32          `json:"code"`
	Status  string          `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func New(baseURL string, httpClient *nethttp.Client) *Client {
	if httpClient == nil {
		httpClient = &nethttp.Client{Timeout: 30 * time.Second}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), httpClient: httpClient}
}

func (c *Client) Download(ctx context.Context, req *helper.DownloadRequest) (*helper.DownloadResponse, error) {
	return c.postDownloadLike(ctx, "/api/v2/download", req)
}

func (c *Client) GenerateViewURL(ctx context.Context, req *helper.DownloadRequest) (*helper.DownloadResponse, error) {
	return c.postDownloadLike(ctx, "/api/v2/generate_view_url", req)
}

func (c *Client) Health(ctx context.Context, _ *helper.HealthRequest) (*helper.HealthResponse, error) {
	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, c.baseURL+"/healthz", nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("health request failed: %s", res.Status)
	}
	return &helper.HealthResponse{Code: helper.CodeOK, Status: "ok", Message: "http health"}, nil
}

func (c *Client) Ready(ctx context.Context, _ *helper.ReadyRequest) (*helper.ReadyResponse, error) {
	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, c.baseURL+"/readyz", nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	status := "ok"
	if res.StatusCode >= 300 {
		status = "unavailable"
	}
	return &helper.ReadyResponse{Code: uint32(res.StatusCode), Status: status, Message: "http ready", Dependencies: map[string]string{"http": status}}, nil
}

func (c *Client) DownloadFile(ctx context.Context, req *helper.DownloadRequest) (*helper.FileDownloadResponse, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"object":  req.Object,
		"bucket":  req.Bucket,
		"path":    req.Path,
		"expires": req.Expires,
	})
	if err != nil {
		return nil, err
	}
	httpReq, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodPost, c.baseURL+"/api/v2/download_file", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	res, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("download_file failed: status=%d", res.StatusCode)
	}
	return &helper.FileDownloadResponse{
		Code:        uint32(res.StatusCode),
		Status:      res.Status,
		Message:     "download file",
		Data:        body,
		ContentType: res.Header.Get("Content-Type"),
	}, nil
}

func (c *Client) View(ctx context.Context, req *helper.ViewRequest) (*helper.ViewResponse, error) {
	if req == nil || req.Path == "" {
		return nil, errors.New("view path is required")
	}
	httpReq, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, c.baseURL+"/api/v2/view?path="+req.Path, nil)
	if err != nil {
		return nil, err
	}
	res, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("view failed: status=%d", res.StatusCode)
	}
	return &helper.ViewResponse{Code: uint32(res.StatusCode), Status: res.Status, Message: "view", Data: body, ContentType: res.Header.Get("Content-Type")}, nil
}

func (c *Client) Upload(ctx context.Context, req *helper.UploadRequest) (*helper.UploadResponse, error) {
	if req == nil {
		return nil, errors.New("upload request is nil")
	}
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("object", req.Filename)
	if err != nil {
		return nil, err
	}
	if _, err = part.Write(req.Content); err != nil {
		return nil, err
	}
	if req.Path != "" {
		_ = writer.WriteField("path", req.Path)
	}
	if req.Bucket != "" {
		_ = writer.WriteField("bucket", req.Bucket)
	}
	if req.Expires > 0 {
		_ = writer.WriteField("expires", strconv.Itoa(int(req.Expires)))
	}
	_ = writer.WriteField("signed", "true")
	if err = writer.Close(); err != nil {
		return nil, err
	}

	httpReq, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodPost, c.baseURL+"/api/v2/upload", body)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("upload failed: status=%d", res.StatusCode)
	}

	env := responseEnvelope{}
	if err = json.Unmarshal(respBody, &env); err != nil {
		return nil, err
	}
	data := struct {
		Filename     string `json:"filename"`
		URL          string `json:"url"`
		PresignedURL string `json:"presigned_url"`
	}{}
	_ = json.Unmarshal(env.Data, &data)
	if data.Filename == "" {
		data.Filename = filepath.Base(req.Filename)
	}

	return &helper.UploadResponse{
		Code:    env.Code,
		Status:  env.Status,
		Message: env.Message,
		Data: &helper.UploadResult{
			Filename:     data.Filename,
			URL:          data.URL,
			PresignedURL: data.PresignedURL,
		},
	}, nil
}

func (c *Client) postDownloadLike(ctx context.Context, endpoint string, req *helper.DownloadRequest) (*helper.DownloadResponse, error) {
	payload, err := json.Marshal(map[string]interface{}{
		"object":  req.Object,
		"bucket":  req.Bucket,
		"path":    req.Path,
		"expires": req.Expires,
	})
	if err != nil {
		return nil, err
	}
	httpReq, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodPost, c.baseURL+endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	res, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	env := responseEnvelope{}
	if err = json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	if res.StatusCode >= 300 {
		msg := env.Message
		if msg == "" {
			msg = res.Status
		}
		return nil, errors.New(msg)
	}
	dataValue := ""
	if len(env.Data) > 0 {
		_ = json.Unmarshal(env.Data, &dataValue)
	}
	return &helper.DownloadResponse{Code: env.Code, Status: env.Status, Message: env.Message, Data: dataValue}, nil
}
