# Testing Guide

## Code Separation and Testability

The codebase follows good separation of concerns, making it highly testable:

### Architecture Layers

1. **Handlers** (`internal/handlers/`)
   - Thin HTTP adapters
   - Parse requests, validate input, handle HTTP concerns
   - Delegate to repositories/services
   - **Testability**: High - can mock repositories/services

2. **Services** (`internal/services/`)
   - Business logic and external integrations
   - OIDC authentication flows
   - **Testability**: Medium-High - some methods make HTTP calls (test with httptest.Server)

3. **Repositories** (`internal/database/`)
   - Data access layer
   - Pure database operations
   - **Testability**: Medium - requires database or sqlmock for unit tests

4. **Models** (`internal/models/`)
   - Data structures and domain types
   - **Testability**: High - pure data, no dependencies

5. **Middleware** (`internal/middleware/`)
   - HTTP middleware (auth, logging, CORS)
   - **Testability**: High - standard http.Handler interface

6. **Config** (`internal/config/`)
   - Configuration loading
   - **Testability**: High - can set environment variables

### Separation Analysis

✅ **Good Separation:**
- Handlers don't contain business logic - they delegate to services/repositories
- Repositories are pure data access - no business logic
- Services contain business logic separated from HTTP concerns
- Models are simple data structures

⚠️ **Areas with External Dependencies:**
- `Provider.GetLoginConfig()` makes HTTP calls to OIDC discovery endpoints
  - Can be tested with `httptest.Server` for integration tests
  - Unit tests can test fallback logic without HTTP
- `JWKSManager` makes HTTP calls to fetch JWKS
  - Cache logic can be tested separately
  - HTTP fetching can use `httptest.Server`

### Testing Strategy

#### Unit Tests (Current Focus)
- ✅ Models - Data validation, JSON serialization
- ✅ Config - Environment variable loading
- ✅ Handlers - Request/response handling with mocks
- ✅ Middleware - HTTP handler behavior
- ✅ Services - Business logic (excluding external HTTP calls)

#### Integration Tests (Future)
- Repositories with real database
- Services with real HTTP endpoints (httptest.Server)
- End-to-end API tests

### Test Coverage Goals

- **Target**: >80% coverage for non-external code
- **Focus**: Unit test all business logic and HTTP handling
- **Exclude**: External HTTP calls (test separately with httptest)

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/handlers/...

# Run with verbose output
go test -v ./...
```

## Test Structure

Tests follow Go conventions:
- Table-driven tests for multiple scenarios
- `t.Parallel()` for independent tests
- Mock dependencies using interfaces or test doubles
- Test both success and error paths
