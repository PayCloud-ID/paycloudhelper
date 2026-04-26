package phtrace

import (
	"testing"
	"time"
)

func TestFromEnv_Defaults(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_ENABLED", "")

	c := FromEnv()
	if c.Enabled {
		t.Errorf("expected Enabled=false when endpoint is empty, got true")
	}
	if c.DialTimeout != 5*time.Second {
		t.Errorf("DialTimeout: want 5s, got %v", c.DialTimeout)
	}
	if c.SamplingRatio != 1.0 {
		t.Errorf("SamplingRatio: want 1.0, got %f", c.SamplingRatio)
	}
}

func TestFromEnv_EnabledWhenEndpointSet(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "svc")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
	t.Setenv("OTEL_ENABLED", "")

	c := FromEnv()
	if !c.Enabled {
		t.Errorf("expected Enabled=true when endpoint set, got false")
	}
}

func TestFromEnv_ExplicitDisable(t *testing.T) {
	t.Setenv("OTEL_SERVICE_NAME", "svc")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
	t.Setenv("OTEL_ENABLED", "false")

	c := FromEnv()
	if c.Enabled {
		t.Errorf("expected Enabled=false when OTEL_ENABLED=false, got true")
	}
}

func TestFromEnv_Options(t *testing.T) {
	c := FromEnv(
		WithServiceName("override"),
		WithSamplingRatio(0.1),
		WithInsecure(false),
	)
	if c.ServiceName != "override" {
		t.Errorf("ServiceName: want override, got %s", c.ServiceName)
	}
	if c.SamplingRatio != 0.1 {
		t.Errorf("SamplingRatio: want 0.1, got %f", c.SamplingRatio)
	}
	if c.Insecure {
		t.Errorf("Insecure: want false, got true")
	}
}

func TestParseResourceAttrs(t *testing.T) {
	got := parseResourceAttrs("service.namespace=pc, region = ap-southeast-1 ,bad=")
	if got["service.namespace"] != "pc" {
		t.Errorf("want pc, got %q", got["service.namespace"])
	}
	if got["region"] != "ap-southeast-1" {
		t.Errorf("want ap-southeast-1, got %q", got["region"])
	}
	if _, ok := got["bad"]; !ok {
		t.Errorf("want bad key present with empty value")
	}
}

func TestTruthy(t *testing.T) {
	cases := map[string]bool{
		"true": true, "TRUE": true, "1": true, "yes": true, "on": true,
		"false": false, "0": false, "no": false, "": false, "nope": false,
	}
	for in, want := range cases {
		if got := truthy(in); got != want {
			t.Errorf("truthy(%q): want %v, got %v", in, want, got)
		}
	}
}

func TestConfig_withDefaults(t *testing.T) {
	c := Config{}.withDefaults()
	if c.DialTimeout <= 0 || c.BatchTimeout <= 0 || c.MetricExportInterval <= 0 {
		t.Errorf("zero durations not backfilled: %+v", c)
	}
	if c.SamplingRatio != 1.0 {
		t.Errorf("SamplingRatio not backfilled: %f", c.SamplingRatio)
	}
	if c.Environment != "dev" {
		t.Errorf("Environment not backfilled: %s", c.Environment)
	}
}
