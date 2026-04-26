package paycloudhelper

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func setupMiniredis(t *testing.T) *miniredis.Miniredis {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(func() {
		resetRedisClientStateForTesting()
		mr.Close()
	})
	resetRedisClientStateForTesting()
	if err := InitializeRedisWithRetry(RedisInitOptions{
		Options:    redis.Options{Addr: mr.Addr()},
		MaxRetries: 1,
		RetryDelay: 10 * time.Millisecond,
		FailFast:   true,
	}); err != nil {
		t.Fatalf("InitializeRedisWithRetry: %v", err)
	}
	return mr
}

func TestVerifCsrf_WithRedis_hitNext(t *testing.T) {
	_ = setupMiniredis(t)
	const token = "csrfvalidtoken01"
	if err := StoreRedis("csrf-"+token, "1", redisDefaultDuration); err != nil {
		t.Fatalf("StoreRedis: %v", err)
	}

	e := echo.New()
	nextOK := false
	h := VerifCsrf(func(c echo.Context) error {
		nextOK = true
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/secure", nil)
	req.Header.Set("X-Xsrf-Token", token)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h(c); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d want 200", rec.Code)
	}
	if !nextOK {
		t.Fatal("expected next handler to run")
	}
}

func TestVerifCsrf_WithRedis_missingKeyUnauthorized(t *testing.T) {
	_ = setupMiniredis(t)

	e := echo.New()
	h := VerifCsrf(func(c echo.Context) error {
		t.Fatal("next should not run")
		return nil
	})

	req := httptest.NewRequest(http.MethodPost, "/secure", nil)
	req.Header.Set("X-Xsrf-Token", "not-in-redis")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h(c); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d want 401", rec.Code)
	}
}

func idemKeyForJSONBody(t *testing.T, body []byte) string {
	t.Helper()
	minified, err := JsonMinify(body)
	if err != nil {
		t.Fatalf("JsonMinify: %v", err)
	}
	sum := md5.Sum(minified)
	return hex.EncodeToString(sum[:])
}

func TestVerifCsrf_RedisConnectionError(t *testing.T) {
	mr := setupMiniredis(t)
	if err := StoreRedis("csrf-goodtoken", "1", redisDefaultDuration); err != nil {
		t.Fatalf("StoreRedis: %v", err)
	}
	mr.Close()

	e := echo.New()
	h := VerifCsrf(func(c echo.Context) error { return nil })
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Xsrf-Token", "goodtoken")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := h(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 for redis transport error, got %d", rec.Code)
	}
}

func TestVerifIdemKey_RedisErrorNotNil(t *testing.T) {
	mr := setupMiniredis(t)
	body := []byte(`{"k":1}`)
	key := idemKeyForJSONBody(t, body)
	// Prime redis then kill it so GetRedis returns a connection error (not redis: nil).
	if err := StoreRedis(key, map[string]int{"k": 1}, redisDefaultDuration); err != nil {
		t.Fatalf("StoreRedis: %v", err)
	}
	mr.Close()

	e := echo.New()
	h := VerifIdemKey(func(c echo.Context) error { return nil })
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("Session", "9")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := h(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("want 500 for redis error, got %d", rec.Code)
	}
}

func TestVerifIdemKey_WithRedis_firstRequestThenDuplicate(t *testing.T) {
	_ = setupMiniredis(t)

	body := []byte(`{"amount":100}`)
	key := idemKeyForJSONBody(t, body)

	e := echo.New()
	calls := 0
	h := VerifIdemKey(func(c echo.Context) error {
		calls++
		return c.NoContent(http.StatusOK)
	})

	run := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/pay", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", key)
		req.Header.Set("Session", "9")
		rec := httptest.NewRecorder()
		ctx := e.NewContext(req, rec)
		if err := h(ctx); err != nil {
			t.Fatalf("handler: %v", err)
		}
		return rec
	}

	r1 := run()
	if r1.Code != http.StatusOK {
		t.Fatalf("first status = %d want 200", r1.Code)
	}
	if calls != 1 {
		t.Fatalf("first calls = %d want 1", calls)
	}

	r2 := run()
	if r2.Code != http.StatusAccepted {
		t.Fatalf("duplicate status = %d want 202", r2.Code)
	}
	if calls != 1 {
		t.Fatalf("after duplicate next calls = %d want 1", calls)
	}
}

func TestVerifCsrf_missingXsrfBadRequest(t *testing.T) {
	e := echo.New()
	h := VerifCsrf(func(c echo.Context) error {
		t.Fatal("next must not run")
		return nil
	})
	req := httptest.NewRequest(http.MethodPost, "/secure", nil)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := h(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rec.Code)
	}
}
