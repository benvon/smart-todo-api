---
name: Structured Logging Security Review
overview: Critical peer review of structured logging implementation focusing on security (injection attacks), Go best practices, and correctness (edge cases). Identifies vulnerabilities and provides actionable fixes.
todos:
  - id: sanitize-url-paths
    content: Sanitize URL paths before logging - remove control characters, truncate length, escape special characters
    status: completed
  - id: fix-panic-error-logging
    content: Fix panic error logging to safely convert to string and sanitize
    status: completed
  - id: add-length-limits
    content: Add maximum length limits to all logged strings (paths, errors, user input)
    status: completed
  - id: sanitize-debug-logs
    content: Apply sanitization even in debug mode for prompts/responses
    status: completed
  - id: fix-duration-field
    content: Fix duration field naming inconsistency (duration_ms vs duration_seconds)
    status: completed
  - id: add-stack-traces
    content: Enable stack traces for error-level logs or add manually for panics
    status: completed
  - id: fix-zap-any-times
    content: Replace zap.Any() with zap.Time() for time.Time values
    status: completed
  - id: add-logger-sync
    content: Add logger.Sync() on application shutdown
    status: completed
  - id: improve-error-context
    content: Add more structured context to error logs (operation, inputs, etc.)
    status: completed
isProject: false
---

# Structured Logging Security Review

## Executive Summary

This review examines the structured logging implementation for security vulnerabilities, Go best practices, and correctness issues. The implementation uses `zap` for structured logging across both app and worker processes.

## Critical Security Issues

### 1. URL Path Injection Vulnerability

**Location**: `internal/middleware/logging.go:24`, `internal/middleware/error.go:29`, `internal/middleware/auth.go:114`, `internal/middleware/audit.go:26`, `internal/middleware/cors.go:60`

**Issue**: User-controlled URL paths are logged directly without sanitization. Malicious actors could inject:

- Control characters (newlines, null bytes)
- JSON injection sequences (`"`, `\`, etc.)
- Log injection attacks to break log parsing
- Extremely long paths causing log buffer overflow

**Example Attack**:

```
GET /api/todos/123%0A%22injected%22%3A%22data%22%7D
```

**Fix**: Sanitize paths before logging:

- Remove or escape control characters
- Truncate to reasonable length (e.g., 500 chars)
- Use `zap.String()` which handles JSON encoding, but add explicit sanitization

### 2. Unsafe Error Logging in Panic Handler

**Location**: `internal/middleware/error.go:28`

**Issue**: `zap.Any("error", err)` logs panic values directly. Panic values can be:

- Arbitrary types (strings, structs, etc.)
- Contain sensitive data (passwords, tokens, user input)
- Cause serialization issues if type doesn't implement proper stringer

**Fix**: Convert errors to strings safely:

```go
var errStr string
if err != nil {
    errStr = fmt.Sprintf("%v", err)
    // Truncate and sanitize
    errStr = sanitizeError(errStr)
}
zap.String("error", errStr)
```

### 3. Full Content Logging in Debug Mode

**Location**: `internal/services/ai/openai.go:135`, `internal/services/ai/openai.go:199`

**Issue**: When `debugMode=true`, full prompts and responses are logged without sanitization. This could expose:

- Sensitive user data
- API keys (if accidentally included)
- PII in conversation history
- Large payloads causing log bloat

**Fix**: Even in debug mode, apply sanitization:

- Truncate to max length (e.g., 10KB)
- Remove or redact sensitive patterns
- Use structured fields with length limits

### 4. Missing Input Validation on Logged Strings

**Location**: Multiple locations using `zap.String()`

**Issue**: No length limits on logged strings. Attackers could:

- Send extremely long values causing memory exhaustion
- Fill log storage quickly
- Cause log aggregation systems to fail

**Fix**: Implement maximum length limits:

- Paths: 500 characters
- User IDs: 128 characters (UUIDs are 36)
- Error messages: 1000 characters
- General strings: 2000 characters

### 5. Inconsistent Sanitization

**Location**: `internal/services/ai/sanitize.go`

**Issue**: `SanitizePrompt` and `SanitizeResponse` only truncate, don't escape:

- Control characters remain
- JSON injection possible if logs are parsed incorrectly
- No validation of content

**Fix**: Add proper sanitization:

- Remove control characters (except newlines in specific contexts)
- Escape JSON special characters if needed
- Validate UTF-8 encoding

## Go Best Practices Issues

### 1. Logger Not Properly Closed

**Location**: Logger initialization (need to check main.go/worker main)

**Issue**: Zap loggers should be synced/flushed before program exit to ensure all logs are written.

**Fix**: Add `defer logger.Sync()` in main functions.

### 2. Inconsistent Error Handling

**Location**: `internal/middleware/error.go:55`

**Issue**: Error logging failures are logged but don't handle the case where logging itself fails (could cause infinite recursion in edge cases).

**Fix**: Use a fallback logger or ensure logging errors don't trigger more logging.

### 3. Missing Context in Logs

**Location**: Various locations

**Issue**: Some log statements lack request/user context, making debugging difficult.

**Fix**: Ensure all logs include:

- Request ID (if available)
- User ID (if authenticated)
- Correlation IDs for tracing

### 4. Logger Initialization Error Not Handled

**Location**: Need to check where logger is created

**Issue**: `NewProductionLogger` returns an error that may not be checked.

**Fix**: Always check logger initialization errors and provide fallback.

## Correctness Issues

### 1. Race Condition in ResponseWriter

**Location**: `internal/middleware/logging.go:17`, `internal/middleware/audit.go:14`

**Issue**: `responseWriter` struct captures status code, but if `WriteHeader` is never called, defaults to 200. However, if `Write` is called first, Go sets status to 200 implicitly, which may not be captured.

**Fix**: Track whether status was explicitly set.

### 2. Duration Field Naming Inconsistency

**Location**: `internal/middleware/logging.go:26`

**Issue**: Field named `duration_ms` but `zap.Duration` encodes as seconds by default (per encoder config).

**Fix**: Either:

- Use `zap.Int64("duration_ms", duration.Milliseconds())` for consistency
- Or rename field to `duration_seconds` and use `zap.Duration`

### 3. Missing Stack Traces for Errors

**Location**: `internal/middleware/error.go`

**Issue**: Panic recovery logs error but no stack trace. Stack traces are disabled in production config.

**Fix**: Enable stack traces for error-level logs in production, or add stack trace manually for panics.

### 4. zap.Any Usage for Time Values

**Location**: `internal/workers/analyzer.go:371`, `internal/workers/tag_analyzer.go:190`

**Issue**: `zap.Any("not_before", job.NotBefore)` logs `*time.Time`. Better to use `zap.Time()` for proper formatting.

**Fix**: Use `zap.Time()` or handle nil pointer safely.

### 5. Incomplete Error Context

**Location**: Various error logging locations

**Issue**: Some errors are logged without sufficient context (what operation failed, what were the inputs, etc.).

**Fix**: Add structured fields for operation context.

## Recommendations

### High Priority

1. **Sanitize all user-controlled input** before logging (paths, headers, query params)
2. **Add length limits** to all logged strings
3. **Fix panic error logging** to safely convert to string
4. **Review debug mode logging** - even debug logs should sanitize sensitive data

### Medium Priority

1. **Standardize duration field** naming and encoding
2. **Add request correlation IDs** throughout the application
3. **Enable stack traces** for error-level logs
4. **Add logger sync** on application shutdown

### Low Priority

1. **Improve error context** in log messages
2. **Use zap.Time()** instead of zap.Any() for time values
3. **Add log sampling** for high-volume endpoints
4. **Consider log levels** - some Debug logs might be Info

## Testing Recommendations

1. **Fuzz testing** for log injection attacks
2. **Load testing** with malicious input to verify length limits
3. **Integration tests** to verify logs are properly formatted and sanitized
4. **Security audit** of log storage and access controls

