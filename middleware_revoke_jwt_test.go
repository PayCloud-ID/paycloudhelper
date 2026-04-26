package paycloudhelper

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

func testRSAPEMPair(t *testing.T) (pubPEM string, priv *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	b := &pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}
	return string(pem.EncodeToMemory(b)), priv
}

func signedRevokeJWT(t *testing.T, priv *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	s, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestRevokeToken_validJWT_noRedisKey_callsNext(t *testing.T) {
	_ = setupMiniredis(t)
	pubPEM, priv := testRSAPEMPair(t)
	t.Setenv("APP_PUBLIC_KEY", pubPEM)

	exp := time.Now().Add(time.Hour).UTC()
	token := signedRevokeJWT(t, priv, jwt.MapClaims{
		"Expired":    exp.Format("2006-01-02 15:04:05"),
		"MerchantId": float64(99),
	})

	e := echo.New()
	nextOK := false
	h := RevokeToken(func(c echo.Context) error {
		nextOK = true
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := h(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d want 200", rec.Code)
	}
	if !nextOK {
		t.Fatal("expected next")
	}
}

func TestRevokeToken_validJWT_revokedInRedis_unauthorized(t *testing.T) {
	_ = setupMiniredis(t)
	pubPEM, priv := testRSAPEMPair(t)
	t.Setenv("APP_PUBLIC_KEY", pubPEM)

	const merchantID = 42
	if err := StoreRedis("revoke_token_42", revokeToken{Status: 4}, redisDefaultDuration); err != nil {
		t.Fatalf("StoreRedis: %v", err)
	}

	exp := time.Now().Add(time.Hour).UTC()
	token := signedRevokeJWT(t, priv, jwt.MapClaims{
		"Expired":    exp.Format("2006-01-02 15:04:05"),
		"MerchantId": float64(merchantID),
	})

	e := echo.New()
	h := RevokeToken(func(c echo.Context) error {
		t.Fatal("next should not run")
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := h(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d want 401", rec.Code)
	}
}

func TestRevokeToken_expiredClaim_unauthorized(t *testing.T) {
	_ = setupMiniredis(t)
	pubPEM, priv := testRSAPEMPair(t)
	t.Setenv("APP_PUBLIC_KEY", pubPEM)

	exp := time.Now().Add(-time.Hour).UTC()
	token := signedRevokeJWT(t, priv, jwt.MapClaims{
		"Expired":    exp.Format("2006-01-02 15:04:05"),
		"MerchantId": float64(1),
	})

	e := echo.New()
	h := RevokeToken(func(c echo.Context) error {
		t.Fatal("next should not run")
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := h(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d want 401", rec.Code)
	}
}

func TestRevokeToken_invalidMerchantClaim_unauthorized(t *testing.T) {
	_ = setupMiniredis(t)
	pubPEM, priv := testRSAPEMPair(t)
	t.Setenv("APP_PUBLIC_KEY", pubPEM)

	exp := time.Now().Add(time.Hour).UTC()
	token := signedRevokeJWT(t, priv, jwt.MapClaims{
		"Expired":    exp.Format("2006-01-02 15:04:05"),
		"MerchantId": "not-a-float",
	})

	e := echo.New()
	h := RevokeToken(func(c echo.Context) error {
		t.Fatal("next should not run")
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	if err := h(ctx); err != nil {
		t.Fatalf("handler: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d want 401", rec.Code)
	}
}
