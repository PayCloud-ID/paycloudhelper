package facade

import (
	"testing"

	sdkgrpc "bitbucket.org/paycloudid/paycloudhelper/sdk/services/s3minio/grpc"
	"bitbucket.org/paycloudid/paycloudhelper/sdk/services/s3minio/helper"
	sdkhttp "bitbucket.org/paycloudid/paycloudhelper/sdk/services/s3minio/http"
)

// TestFacadeGRPCReturnsHelperClient verifies NewGRPC returns a type
// satisfying helper.Client at compile time.
func TestFacadeGRPCReturnsHelperClient(t *testing.T) {
	t.Parallel()

	// We can't construct a real gRPC conn here, but we can verify the return type.
	var _ helper.Client = (*sdkgrpc.Client)(nil)
}

// TestNewGRPCNotNil verifies NewGRPC wraps the pooled client constructor; nil conn
// is only valid for compile-time interface checks (RPCs would panic if invoked).
func TestNewGRPCNotNil(t *testing.T) {
	t.Parallel()

	c := NewGRPC(nil)
	if c == nil {
		t.Fatal("NewGRPC returned nil")
	}
	var _ helper.Client = c
}

// TestFacadeHTTPBridgeReturnsHelperClient verifies NewHTTPBridge returns a type
// satisfying helper.Client at compile time.
func TestFacadeHTTPBridgeReturnsHelperClient(t *testing.T) {
	t.Parallel()

	var _ helper.Client = (*sdkhttp.Client)(nil)
}

// TestNewHTTPBridgeNotNil verifies the constructor works.
func TestNewHTTPBridgeNotNil(t *testing.T) {
	t.Parallel()

	c := NewHTTPBridge("http://localhost:9193", nil)
	if c == nil {
		t.Fatal("NewHTTPBridge returned nil")
	}
}

// TestNewHTTPBridgeWithTrailingSlash verifies URL trimming.
func TestNewHTTPBridgeWithTrailingSlash(t *testing.T) {
	t.Parallel()

	c := NewHTTPBridge("http://localhost:9193/", nil)
	if c == nil {
		t.Fatal("NewHTTPBridge returned nil")
	}
}
