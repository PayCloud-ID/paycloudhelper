package phlogger

import "testing"

func TestNewLogContext_Empty(t *testing.T) {
	ctx := NewLogContext()
	if ctx.prefix != "" {
		t.Fatalf("empty context should have no prefix, got %q", ctx.prefix)
	}
}

func TestNewLogContext_SinglePair(t *testing.T) {
	ctx := NewLogContext("req_id", "abc-123")
	want := "[req_id=abc-123] "
	if ctx.prefix != want {
		t.Fatalf("got %q, want %q", ctx.prefix, want)
	}
}

func TestNewLogContext_MultiplePairs(t *testing.T) {
	ctx := NewLogContext("req_id", "abc", "merchant", "M001", "user", "U42")
	want := "[req_id=abc merchant=M001 user=U42] "
	if ctx.prefix != want {
		t.Fatalf("got %q, want %q", ctx.prefix, want)
	}
}

func TestNewLogContext_OddFieldsDropsTrailing(t *testing.T) {
	// Odd number of fields: trailing key is silently dropped.
	ctx := NewLogContext("req_id", "abc", "orphan")
	want := "[req_id=abc] "
	if ctx.prefix != want {
		t.Fatalf("got %q, want %q", ctx.prefix, want)
	}
}

func TestLogContext_With_AddsFields(t *testing.T) {
	parent := NewLogContext("req_id", "abc")
	child := parent.With("step", "validate")
	want := "[req_id=abc step=validate] "
	if child.prefix != want {
		t.Fatalf("got %q, want %q", child.prefix, want)
	}
}

func TestLogContext_With_EmptyFields(t *testing.T) {
	parent := NewLogContext("req_id", "abc")
	same := parent.With()
	if same != parent {
		t.Fatal("With() with no fields should return same context")
	}
}

func TestLogContext_With_EmptyParent(t *testing.T) {
	parent := NewLogContext()
	child := parent.With("step", "init")
	want := "[step=init] "
	if child.prefix != want {
		t.Fatalf("got %q, want %q", child.prefix, want)
	}
}

func TestLogContext_With_Chained(t *testing.T) {
	ctx := NewLogContext("req_id", "abc").With("step", "1").With("detail", "x")
	want := "[req_id=abc step=1 detail=x] "
	if ctx.prefix != want {
		t.Fatalf("got %q, want %q", ctx.prefix, want)
	}
}

func TestLogContext_LogI_EmitsWithPrefix(t *testing.T) {
	// Ensure LogI doesn't panic and integrates with sampler.
	// We can't capture output easily but verify no panic/race.
	saved := globalSampler
	defer func() { globalSampler = saved }()
	InitializeSampler(SamplerConfig{}) // disabled — all pass through

	ctx := NewLogContext("test", "context")
	ctx.LogI("payment processed id=%s", "P123")
	ctx.LogE("something failed: %s", "timeout")
	ctx.LogW("slow query: %dms", 500)
	ctx.LogD("debug detail: %v", map[string]int{"a": 1})
}

func TestLogContext_ImmutablePrefix(t *testing.T) {
	// Verify that creating a child doesn't modify parent.
	parent := NewLogContext("req_id", "abc")
	parentPrefix := parent.prefix
	_ = parent.With("extra", "field")
	if parent.prefix != parentPrefix {
		t.Fatal("parent prefix was mutated by With()")
	}
}
