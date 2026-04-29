package grpc

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/PayCloud-ID/paycloudhelper/sdk/services/s3minio/helper"
	"github.com/PayCloud-ID/paycloudhelper/sdk/services/s3minio/pb"
)

func normalizeStatus(status string) string {
	if strings.TrimSpace(strings.ToLower(status)) == "ok" {
		return "ok"
	}
	return strings.TrimSpace(status)
}

func (c *Client) Health(ctx context.Context, req *helper.HealthRequest) (*helper.HealthResponse, error) {
	_ = req
	started := time.Now()
	res, err := c.pb.Health(ctx, &pb.HealthRequest{})
	if err != nil {
		return nil, err
	}
	status := ""
	if res != nil {
		status = normalizeStatus(res.Status)
	}
	code := uint32(pb.OKCode())
	if status != "ok" {
		code = uint32(http.StatusServiceUnavailable)
	}
	return &helper.HealthResponse{Code: code, Status: status, Message: "grpc health"}, helper.ObserveProbe(started, "health", "grpc", code)
}

func (c *Client) Ready(ctx context.Context, req *helper.ReadyRequest) (*helper.ReadyResponse, error) {
	_ = req
	started := time.Now()
	health, err := c.Health(ctx, &helper.HealthRequest{})
	if err != nil {
		return nil, err
	}
	ready := &helper.ReadyResponse{
		Code:    health.Code,
		Status:  health.Status,
		Message: "grpc readiness",
		Dependencies: map[string]string{
			"grpc": health.Status,
		},
	}
	if ready.Status == "" {
		ready.Status = "unavailable"
	}
	return ready, helper.ObserveProbe(started, "ready", "grpc", ready.Code)
}

func CheckHealth(ctx context.Context, p helper.HealthProber) (*helper.HealthResponse, error) {
	return helper.CheckHealth(ctx, p)
}

func CheckReady(ctx context.Context, p helper.ReadinessProber) (*helper.ReadyResponse, error) {
	return helper.CheckReady(ctx, p)
}
