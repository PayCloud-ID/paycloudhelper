# Testing & Validation — paycloudhelper

## Test Commands

```bash
# All tests — required before every commit
go test ./...

# With race detector — required for init/concurrency changes
go test -race ./...

# Single package
go test -v ./...

# Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## Test File Inventory

| Test File | Covers |
|-----------|--------|
| `config_test.go` | `ValidateConfiguration()`, env var handling |
| `headers_test.go` | `Headers` struct validation |
| `helpers_test.go` | `JSONEncode`, `GetOrGenerateRequestID`, misc utils |
| `middleware_test.go` | `VerifCsrf`, `VerifIdemKey`, `RevokeToken` |
| `redis_test.go` | Redis store/get, lock acquire/release |
| `response_test.go` | `ResponseApi` methods (Success, BadRequest, etc.) |

## Coverage Requirements

- **New features**: Must include unit test covering happy path + error path
- **Bug fixes**: Must add test that reproduces the bug (regression guard)
- **Middleware changes**: Must test valid + invalid header permutations + Redis failure simulation
- **Init changes**: Run with `-race` flag to catch data races

## Middleware Test Pattern

```go
func TestVerifCsrf(t *testing.T) {
    e := echo.New()
    
    // Happy path: valid CSRF token in Redis
    req := httptest.NewRequest(http.MethodGet, "/", nil)
    req.Header.Set("X-Xsrf-Token", "valid-token")
    rec := httptest.NewRecorder()
    c := e.NewContext(req, rec)
    
    // Stub Redis (or use real Redis in integration test)
    // ...
    
    handler := VerifCsrf(func(c echo.Context) error {
        return c.JSON(200, "ok")
    })
    
    if assert.NoError(t, handler(c)) {
        assert.Equal(t, http.StatusOK, rec.Code)
    }
}
```

## Load Testing (Middleware Changes)

Any change to `VerifCsrf`, `VerifIdemKey`, or `RevokeToken` requires:
- Minimum **1000 req/s** for **5 minutes** against a staging service
- No error rate increase above baseline
- No memory leak (flat heap profile over 5min run)

## Build Validation

```bash
# Ensure library compiles without errors
go build ./...

# Vet for common issues
go vet ./...

# Tidy dependencies
go mod tidy

# Check for unused/indirect deps
go mod verify
```

## Pre-Commit Checklist

- [ ] `go test ./...` passes
- [ ] `go test -race ./...` passes (for any concurrency changes)
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` no issues
- [ ] New code has corresponding test cases
- [ ] Public API matches versioning intent (patch/minor/major)
