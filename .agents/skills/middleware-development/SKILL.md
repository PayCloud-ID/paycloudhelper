---
name: middleware-development
description: Guides creation and modification of Echo HTTP middleware in paycloudhelper, including response helpers and JSON library selection.
applyTo: '**/csrf.go, **/idempotency-key.go, **/revoke-token.go'
---
# Middleware Development — paycloudhelper

## Overview

Middleware in paycloudhelper wraps Echo `HandlerFunc`. Each middleware: validates request headers → checks Redis/JWT → calls `next(c)` or returns error response.

## File Convention

```
{feature}.go          # Middleware implementation
{feature}_test.go     # Tests (required)
```

## Echo Middleware Template

```go
func MyMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        var response ResponseApi

        // 1. Get/generate request ID for tracing
        requestID := GetOrGenerateRequestID(c.Request().Header.Get("X-Request-ID"))
        c.Request().Header.Set("X-Request-ID", requestID)
        c.Response().Header().Set("X-Request-ID", requestID)

        // 2. Extract headers
        myHeader := c.Request().Header.Get("My-Header")
        if myHeader == "" {
            LogE("[%s] MyMiddleware: missing header", requestID)
            response.BadRequest("missing header", "MY_MISSING_HEADER")
            return c.JSON(response.Code, response)
        }

        // 3. Validate (Redis, DB, JWT)
        // ...

        // 4. Pass to handler
        LogD("[%s] MyMiddleware: validated successfully", requestID)
        return next(c)
    }
}
```

## Existing Middleware Reference

### `VerifCsrf` (`csrf.go`)

- **Header:** `X-Xsrf-Token`
- **Lookup:** Redis key `csrf-{token}` — if not found → `401 Unauthorized`
- **Expiry:** Based on `Session` header (default 9s)
- **Log pattern:** `[{requestID}] VerifCsrf: ...`

### `VerifIdemKey` (`idempotency-key.go`)

- **Headers:** `Idempotency-Key`, `Session`
- **Dedup:** MD5 hash of body → stored in Redis for `Session` seconds
- **Duplicate found:** Returns `202 Accepted` (not an error)
- **JSON parsing:** Uses `jsoniter.ConfigFastest` for performance on hot path
- **Log pattern:** `[{requestID}] VerifIdemKey: ...`

### `RevokeToken` (`revoke-token.go`)

- **Header:** `Authorization` (Bearer JWT)
- **RSA key:** From `APP_PUBLIC_KEY` env var
- **Revoke check:** Redis key `revoke_token_{merchantId}` — if exists → `401 Unauthorized`
- **JWT lib:** `github.com/golang-jwt/jwt/v5`
- **Log pattern:** `[{requestID}] RevokeToken: ...`

## Response Helpers (`response.go`)

```go
var response ResponseApi

response.Success("ok", data)           // 200
response.Accepted(data)                // 202
response.BadRequest("msg", "ERR_CODE") // 400 — also logs via LoggerErrorHub
response.Unauthorized("msg", "")       // 401 — also logs via LoggerErrorHub
response.InternalServerError(err)      // 500 — also logs via LoggerErrorHub

return c.JSON(response.Code, response)
```

## JSON Library Selection in Middleware

| Scenario | Library | Reason |
|----------|---------|--------|
| Hot-path body parsing (per-request) | `jsoniter.ConfigFastest` | Performance |
| Storing to Redis | `encoding/json` | Compatibility |
| Logging/debugging | `encoding/json` helpers (`LogJ`, `LogJI`) | Readability |
| Consumer opt-in throughput | `phjson` (Sonic) | Maximum perf |

## Header Validation Pattern (`headers.go`)

```go
header := &Headers{
    Csrf:           c.Request().Header.Get("X-Xsrf-Token"),
    IdempotencyKey: c.Request().Header.Get("Idempotency-Key"),
    Session:        c.Request().Header.Get("Session"),
    RequestID:      requestID,
}
validate := header.ValiadateHeaderCsrf()  // or ValiadateHeaderIdem()
if validate != nil {
    response.BadRequest("invalid validation", "")
    return c.JSON(response.Code, response)
}
```

## Error Catalog

| Error | Code | Middleware | Cause |
|-------|------|-----------|-------|
| Missing CSRF token | `400` | `VerifCsrf` | Blank `X-Xsrf-Token` header |
| CSRF token not in Redis | `401` | `VerifCsrf` | Token expired or invalid |
| Redis error (CSRF) | `500` | `VerifCsrf` | Redis connection issue |
| Invalid idempotency key | `400 IDEM_INVALID_FORMAT` | `VerifIdemKey` | Malformed key format |
| Duplicate request | `202` | `VerifIdemKey` | Same payload within session window |
| Invalid JWT | `401` | `RevokeToken` | Expired or tampered token |
| Revoked token | `401` | `RevokeToken` | `revoke_token_{id}` exists in Redis |

## Common Mistakes

```go
// ❌ BAD: Allocates config on every request
func VerifCsrf(next echo.HandlerFunc) echo.HandlerFunc {
    config := loadConfig()  // Don't do this per-request
    return func(c echo.Context) error { ... }
}

// ❌ BAD: Panic if Redis is nil
func MyMiddleware(...) {
    result := GetRedis("key")  // panic if Redis not initialized
}

// ✅ GOOD: Guard against uninitialized Redis
client, err := GetRedisPoolClient()
if err != nil {
    response.InternalServerError(err)
    return c.JSON(response.Code, response)
}
```
