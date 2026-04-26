package paycloudhelper

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestVerifIdemKey_invalidSessionBadRequest(t *testing.T) {
	_ = setupMiniredis(t)
	body := []byte(`{"a":1}`)
	key := idemKeyForJSONBody(t, body)

	e := echo.New()
	h := VerifIdemKey(func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("Session", "not-a-number")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := h(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rec.Code)
	}
}

func TestVerifIdemKey_invalidIdempotencyKeyFormat(t *testing.T) {
	_ = setupMiniredis(t)
	e := echo.New()
	h := VerifIdemKey(func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "bad@chars#")
	req.Header.Set("Session", "9")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := h(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", rec.Code)
	}
}

func TestVerifIdemKey_md5KeyMismatch(t *testing.T) {
	_ = setupMiniredis(t)
	body := []byte(`{"amount":7}`)
	wrongKey := "0000000000000000000000000000000"

	e := echo.New()
	h := VerifIdemKey(func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/pay", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", wrongKey)
	req.Header.Set("Session", "9")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := h(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400 for MD5 mismatch", rec.Code)
	}
}

func TestVerifIdemKey_duplicateCorruptCachedPayload(t *testing.T) {
	mr := setupMiniredis(t)
	body := []byte(`{"dup":true}`)
	key := idemKeyForJSONBody(t, body)

	e := echo.New()
	calls := 0
	h := VerifIdemKey(func(c echo.Context) error {
		calls++
		return c.NoContent(http.StatusOK)
	})

	run := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/idem", bytes.NewReader(body))
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

	if run().Code != http.StatusOK {
		t.Fatalf("first request should succeed")
	}
	if calls != 1 {
		t.Fatalf("calls=%d want 1", calls)
	}

	if err := mr.Set(key, "not-json"); err != nil {
		t.Fatalf("corrupt redis key: %v", err)
	}

	rec := run()
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("duplicate with corrupt cache: status=%d want 500", rec.Code)
	}
}

func TestVerifIdemKey_shortSessionNormalized(t *testing.T) {
	// Session values < 4 are normalized to 9 (TTL multiplier) in VerifIdemKey.
	_ = setupMiniredis(t)
	body := []byte(`{"solo":true}`)
	key := idemKeyForJSONBody(t, body)

	e := echo.New()
	h := VerifIdemKey(func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/n", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("Session", "2")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := h(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
}

type errReadBody struct{}

func (errReadBody) Read(p []byte) (int, error) {
	return 0, errors.New("read failed")
}

func TestReadBody_applicationJSON_bodyReadError(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Body = io.NopCloser(errReadBody{})
	req.Header.Set("Content-Type", "application/json")
	c := e.NewContext(req, httptest.NewRecorder())
	_, _, err := ReadBody(c, "x")
	if err == nil || err.Error() != "read failed" {
		t.Fatalf("err=%v", err)
	}
}

func TestReadBody_applicationJSON_invalidJSON(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte(`not json`)))
	req.Header.Set("Content-Type", "application/json")
	c := e.NewContext(req, httptest.NewRecorder())
	_, _, err := ReadBody(c, "ab")
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

// idempotencyKeyForEchoMultipartForm matches VerifIdemKey body hashing: Echo Bind on
// multipart form, json.Marshal(request), then JsonMinify + MD5 (same as ReadBody).
func idempotencyKeyForEchoMultipartForm(t *testing.T, e *echo.Echo, formBody []byte, contentType string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/probe", bytes.NewReader(formBody))
	req.Header.Set("Content-Type", contentType)
	c := e.NewContext(req, httptest.NewRecorder())
	var m map[string]interface{}
	if err := c.Bind(&m); err != nil {
		t.Fatalf("bind: %v", err)
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	min, err := JsonMinify(raw)
	if err != nil {
		t.Fatalf("minify: %v", err)
	}
	sum := md5.Sum(min)
	return hex.EncodeToString(sum[:])
}

func TestVerifIdemKey_multipartForm(t *testing.T) {
	_ = setupMiniredis(t)
	e := echo.New()

	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	if err := mw.WriteField("solo", "true"); err != nil {
		t.Fatal(err)
	}
	ct := mw.FormDataContentType()
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	payload := buf.Bytes()
	key := idempotencyKeyForEchoMultipartForm(t, e, payload, ct)

	h := VerifIdemKey(func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, "/m", bytes.NewReader(payload))
	req.Header.Set("Content-Type", ct)
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("Session", "9")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := h(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200", rec.Code)
	}
}
