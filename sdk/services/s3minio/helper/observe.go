package helper

import "time"

// ProbeObserveFunc is an optional hook for health/readiness probe latency and outcomes.
// Services may assign this at init (e.g. to emit metrics or structured logs) without
// importing the concrete gRPC adapter. When nil, probes incur no extra work.
var ProbeObserveFunc func(started time.Time, probeName string, transport string, code uint32)

// ObserveProbe invokes ProbeObserveFunc when set. Always returns nil so adapters
// keep a stable (result, error) signature for Health/Ready.
func ObserveProbe(started time.Time, probeName, transport string, code uint32) error {
	if ProbeObserveFunc != nil {
		ProbeObserveFunc(started, probeName, transport, code)
	}
	return nil
}
