# Logging — paycloudhelper

The `paycloudhelper` library provides a structured, consistent logging API for all PayCloud microservices. It wraps `[kataras/golog](https://github.com/kataras/golog)` with pre-configured format and level shortcuts.

## Package Architecture

```
paycloudhelper (root)      ← consumer-facing API
    └── phlogger/          ← implementation (kataras/golog wrapper)
           └── phhelper/   ← JSON helpers (ToJson, ToJsonIndent)
```

Consumer services import the **root package** only and use the aliased shorthand functions.

---

## Installation & Import

```go
import pchelper "github.com/PayCloud-ID/paycloudhelper"
```

Always alias as `pchelper` for consistency across all PayCloud services.

---

## Auto-Initialization

The logger is initialized automatically when the package is imported via `init()`:

```go
// Happens automatically — no explicit call needed
func init() {
    InitializeLogger() // sets time format: "2006-01-02 15:04:05.000", level: "info"
}
```

The default log level is `**info**`, meaning `LogD` (debug) calls are **silent** in production.

---

## API Reference

### Root Package (`pchelper`)


| Function               | Signature                                  | Description                                   |
| ---------------------- | ------------------------------------------ | --------------------------------------------- |
| `pchelper.LogI`        | `LogI(format string, args ...interface{})` | Info level — normal operations                |
| `pchelper.LogE`        | `LogE(format string, args ...interface{})` | Error level — failures and errors             |
| `pchelper.LogW`        | `LogW(format string, args ...interface{})` | Warning level — degraded but recoverable      |
| `pchelper.LogD`        | `LogD(format string, args ...interface{})` | Debug level — verbose, silent at `info` level |
| `pchelper.LogF`        | `LogF(format string, args ...interface{})` | Fatal — logs then exits process               |
| `pchelper.LogJ`        | `LogJ(arg interface{})`                    | Logs any value as compact JSON at Info level  |
| `pchelper.LogJI`       | `LogJI(arg interface{})`                   | Logs any value as indented JSON at Info level |
| `pchelper.LogErr`      | `LogErr(err error)`                        | Logs an error value at Error level            |
| `pchelper.LogSetLevel` | `LogSetLevel(levelName string)`            | Change active log level at runtime            |


### Direct Logger Access

```go
pchelper.Log          // *golog.Logger — full golog instance
pchelper.GinLevel     // golog.Level = 6, custom "gin"/"http-server" level
```

---

## Log Level Selection Guide

```
Is the message about a failure or unexpected error?
├── yes → LogE  (gRPC failure, DB error, nil pointer, validation failure)
└── no
    Is the state degraded but operation can continue?
    ├── yes → LogW  (retry, fallback, deprecated call)
    └── no
        Is this detailed tracing for debugging?
        ├── yes → LogD  (Redis ops, gRPC calls, intermediate values)
        └── no → LogI   (startup, completion, state changes, audit events)
```

---

## Usage Examples

### Basic Levels

```go
import pchelper "github.com/PayCloud-ID/paycloudhelper"

// Info — normal operation
pchelper.LogI("[InitRedis] connected to Redis host=%s port=%s", host, port)

// Error — failure with context
pchelper.LogE("[GetMerchant] gRPC error merchant_code=%s err=%v", code, err)

// Warning — recoverable issue
pchelper.LogW("[ProcessBatch] retry attempt=%d max=%d", attempt, maxRetries)

// Debug — verbose tracing (silent at default info level)
pchelper.LogD("[BuildRequest] payload merchant_id=%s amount=%d", id, amount)
```

### Function Name Convention

Every log line must include the calling function name in square brackets:

```go
func InitDatabase() error {
    pchelper.LogI("[InitDatabase] connecting to DB host=%s", host)
    if err != nil {
        pchelper.LogE("[InitDatabase] connection failed err=%v", err)
        return err
    }
    pchelper.LogI("[InitDatabase] connected successfully")
    return nil
}
```

### JSON Object Logging

```go
type Order struct {
    ID     string
    Amount int
    Status string
}

order := Order{ID: "ORD-123", Amount: 50000, Status: "pending"}

// Compact (single line)
pchelper.LogJ(order)
// Output: {"ID":"ORD-123","Amount":50000,"Status":"pending"}

// Indented (human-readable)
pchelper.LogJI(order)
// Output:
// {
//   "ID": "ORD-123",
//   "Amount": 50000,
//   "Status": "pending"
// }
```

### Changing Log Level

```go
func main() {
    // Enable debug logging (e.g., in development)
    if os.Getenv("APP_ENV") == "develop" {
        pchelper.LogSetLevel("debug")
    }
}
```

Valid levels: `"debug"`, `"info"`, `"warn"`, `"error"`, `"fatal"`.

---

## Output Format

The logger outputs to stdout with the format:

```
[LEVEL] 2006-01-02 15:04:05.000 [FunctionName] message key=value
```

Example:

```
[INFO] 2026-02-27 10:05:23.481 [InitRedis] connected host=redis:6379 db=0
[ERRO] 2026-02-27 10:05:24.012 [GetMerchant] gRPC error merchant_code=MRC001 err=connection refused
[WARN] 2026-02-27 10:05:25.330 [ProcessBatch] retry attempt=1 max=3
```

## Prefix Builder (Global)

`paycloudhelper` uses a shared prefix builder from `phhelper` to standardize function tags in log messages:

- `phhelper.LogModulePrefix` (global const, default: `"pchelper"`)
- `phhelper.BuildLogPrefix(functionName)`

Behavior:

- If `LogModulePrefix` is not empty → prefix becomes `[pchelper.FunctionName]`
- If `LogModulePrefix` is empty → prefix becomes `[FunctionName]`

Example:

```go
prefix := phhelper.BuildLogPrefix("InitializeRedisWithRetry")
pchelper.LogE("%s failed to initialize redis err=%v", prefix, err)
```

Possible output:

- with non-empty module prefix: `[pchelper.InitializeRedisWithRetry] failed to initialize redis err=...`
- with empty module prefix: `[InitializeRedisWithRetry] failed to initialize redis err=...`

---

## Implementing in a New Service

### 1. Add dependency

```bash
go get github.com/PayCloud-ID/paycloudhelper@latest
```

### 2. Import with alias

```go
import pchelper "github.com/PayCloud-ID/paycloudhelper"
```

### 3. Replace existing log calls


| Before                     | After                                        |
| -------------------------- | -------------------------------------------- |
| `fmt.Println("started")`   | `pchelper.LogI("[FuncName] started")`        |
| `fmt.Printf("val: %s", v)` | `pchelper.LogI("[FuncName] val=%s", v)`      |
| `fmt.Println(err)`         | `pchelper.LogE("[FuncName] error: %v", err)` |
| `log.Println("msg")`       | `pchelper.LogI("[FuncName] msg")`            |
| `log.Printf("msg %s", v)`  | `pchelper.LogI("[FuncName] msg %s", v)`      |
| `log.Fatal(err)`           | `pchelper.LogF("[FuncName] fatal: %v", err)` |
| `log.Println(struct)`      | `pchelper.LogJ(struct)`                      |


### 4. Verify build passes

```bash
go build ./...
```

---

## Anti-Patterns

```go
// ❌ Missing function name
pchelper.LogE("error: %v", err)

// ❌ String concatenation instead of format
pchelper.LogE("[Func] err=" + err.Error())

// ❌ Wrong level (error on info path)
pchelper.LogE("[GetData] data fetched successfully")

// ❌ Using stdlib log alongside pchelper
import "log"
log.Println("debug")

// ❌ Logging sensitive data (passwords, tokens)
pchelper.LogD("[Auth] password=%s", password)

// ✅ Correct
pchelper.LogE("[Func] operation failed key=%s err=%v", key, err)
pchelper.LogI("[Func] operation completed key=%s", key)
pchelper.LogD("[Func] redis get key=%s", key)
```

---

## phlogger Package (Internal)

The `phlogger` subpackage is the implementation layer and is **not intended for direct use** by consumer services. It is re-exported through the root `paycloudhelper` package.

```go
// phlogger provides:
// - Global *golog.Logger instance
// - Level aliases (LogD, LogI, LogW, LogE, LogF)
// - Custom "gin" log level (level 6, green color)
// - InitializeLogger(): sets time format + default level
// - LogJ / LogJI: JSON helpers via phhelper.ToJson/ToJsonIndent
// - LogErr(err error): error-typed shorthand
```

If you must import `phlogger` directly (e.g., from another subpackage):

```go
import "github.com/PayCloud-ID/paycloudhelper/phlogger"

phlogger.LogI("[FuncName] msg")
phlogger.LogE("[FuncName] error: %v", err)
```

