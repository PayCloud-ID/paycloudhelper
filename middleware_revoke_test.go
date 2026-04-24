package paycloudhelper

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestRevokeToken_invalidAuthorizationParts(t *testing.T) {
	e := echo.New()
	h := RevokeToken(func(c echo.Context) error {
		t.Fatal("next should not run")
		return nil
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "onlyonepart")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := h(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestRevokeToken_wrongBearerPrefix(t *testing.T) {
	e := echo.New()
	h := RevokeToken(func(c echo.Context) error { return nil })
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Basic xxx")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)
	if err := h(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", rec.Code)
	}
}
