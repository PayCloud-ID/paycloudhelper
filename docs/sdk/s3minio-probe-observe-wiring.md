# S3MinIO gRPC probe observation hook

The SDK’s gRPC health adapter calls `helper.ObserveProbe` after each `Health` / `Ready` check. Consumers can assign `helper.ProbeObserveFunc` once at process startup (after logging is initialized) to record latency and status codes without importing the gRPC package.

## Example (`paycloud-be-adminpg-manager/main.go`)

Wire the hook immediately after `initLogger()` so `pchelper.LogD` is available:

```go
import (
	"time"

	pchelper "github.com/PayCloud-ID/paycloudhelper"
	s3helper "github.com/PayCloud-ID/paycloudhelper/sdk/services/s3minio/helper"
)

func main() {
	initLogger()
	initS3MinioProbeObserve()
	initSentry()
	// ...
}

func initS3MinioProbeObserve() {
	s3helper.ProbeObserveFunc = func(started time.Time, probeName, transport string, code uint32) {
		pchelper.LogD("[S3MinioProbe] probe=%s transport=%s code=%d latency_ms=%d",
			probeName, transport, code, time.Since(started).Milliseconds())
	}
}
```

When `ProbeObserveFunc` is nil, probes add no extra work. Use this for metrics, tracing, or sampled debug logs tied to MinIO manager connectivity checks.
