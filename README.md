# Smart Todo - LLM-Integrated Todo List Manager

A simple yet powerful todo list manager with a Go REST API backend, PostgreSQL database, static web frontend, and future OpenAI integration for smart categorization and metadata tagging.

## Overview

Smart Todo is designed to be simple and intuitive, allowing users to quickly input todo items while the backend handles categorization and metadata. The application uses OIDC authentication (currently supporting AWS Cognito) and provides a clean three-column interface organizing todos by time horizon: Now, Soon, and Later.

## Architecture

- **Backend**: Go REST API server with PostgreSQL database, API versioning (`/api/v1/`), and OpenAPI contract
- **Frontend**: Static Progressive Web App (PWA) that can be built and deployed independently
- **Authentication**: OIDC-based authentication using JWT tokens (AWS Cognito first)
- **API Contract**: OpenAPI 3.0 specification maintained for frontend/backend coordination
- **CLI Tool**: Configuration tool for setting up OIDC/Cognito authentication

## Prerequisites

- **Go 1.23+** - [Install Go](https://go.dev/doc/install)
- **PostgreSQL 12+** - [Install PostgreSQL](https://www.postgresql.org/download/)
- **Redis 7+** - [Install Redis](https://redis.io/download) - Required for rate limiting
- **Node.js** (optional, for frontend build tooling if needed)
- **golang-migrate** - Database migration tool: [Install migrate](https://github.com/golang-migrate/migrate)
- **AWS Cognito** (or other OIDC provider) - For authentication

## Dependencies

This project uses the following key dependencies:

- **github.com/go-chi/httprate** - Rate limiting middleware interface
- **github.com/redis/go-redis/v9** - Redis client for distributed rate limiting
- **github.com/go-playground/validator/v10** - Input validation library
- **github.com/lestrrat-go/jwx/v2** - JWT token verification
- **github.com/gorilla/mux** - HTTP router
- **github.com/lib/pq** - PostgreSQL driver

See `go.mod` for the complete dependency list.

## Getting Started

### 1. Clone the Repository

```bash
git clone <repository-url> smart-todo
cd smart-todo
```

### 2. Install Dependencies

```bash
go mod tidy
```

### 3. Set Up Database

Create a PostgreSQL database:

```bash
createdb smarttodo
# Or using psql:
# psql -U postgres -c "CREATE DATABASE smarttodo;"
```

### 5. Run Database Migrations

```bash
# Set your database URL
export DATABASE_URL="postgres://user:password@localhost/smarttodo?sslmode=disable"

# Run migrations
migrate -path internal/database/migrations -database "$DATABASE_URL" up
```

### 6. Configure Environment Variables

Create a `.env` file or set environment variables:

```bash
export DATABASE_URL="postgres://user:password@localhost/smarttodo?sslmode=disable"
export REDIS_URL="redis://localhost:6379/0"  # Required for rate limiting
export SERVER_PORT="8080"
export BASE_URL="http://localhost:8080"
export FRONTEND_URL="http://localhost:3000"
export OPENAI_API_KEY=""  # For Phase 2
# Security settings (optional for local development)
# export ENABLE_HSTS="false"  # Don't set HSTS for local HTTP development
# export OIDC_PROVIDER="cognito"  # Default provider name
```

### 7. Configure OIDC (AWS Cognito or Other Provider)

Use the CLI tool to configure OIDC authentication. The provider name can be any identifier:

```bash
# Build the configure tool
go build -o bin/smart-todo-configure ./cmd/configure

# Configure AWS Cognito (public client - no secret required)
./bin/smart-todo-configure oidc cognito \
  --issuer "https://cognito-idp.<region>.amazonaws.com/<pool-id>" \
  --client-id "<your-client-id>" \
  --redirect-uri "http://localhost:3000/index.html"

# Or for a confidential client (with secret)
./bin/smart-todo-configure oidc cognito \
  --issuer "https://cognito-idp.<region>.amazonaws.com/<pool-id>" \
  --client-id "<your-client-id>" \
  --client-secret "<your-client-secret>" \
  --redirect-uri "http://localhost:3000/index.html"

# Example: Configure a different OIDC provider
./bin/smart-todo-configure oidc okta \
  --issuer "https://dev-123456.okta.com/oauth2/default" \
  --client-id "<your-client-id>" \
  --redirect-uri "http://localhost:3000/index.html"

# List configured providers
./bin/smart-todo-configure list

# Test configuration (use the provider name you configured)
./bin/smart-todo-configure test --provider cognito
```

**Note**: The provider name used in the `oidc <provider-name>` command should match the `OIDC_PROVIDER` environment variable (defaults to `cognito`). You can configure multiple providers, but only one will be active at a time based on the `OIDC_PROVIDER` setting.

### 8. Build and Run the Backend

```bash
# Build the server
go build -o bin/smart-todo-server ./cmd/server

# Run the server
./bin/smart-todo-server

# Or run directly
go run ./cmd/server
```

The API will be available at `http://localhost:8080`

### 8. Set Up and Run the Frontend

The frontend is a static web application that can be served by any static file server.

#### Development Setup

```bash
# Configure API base URL
cp web/config.json.example web/config.json
# Edit web/config.json and set the api_base_url to your backend URL

# Serve using any static file server, for example:
cd web

# Using Python 3
python3 -m http.server 3000

# Using Node.js http-server
npx http-server -p 3000

# Using Go http server
go run -m http.server 3000
```

The frontend will be available at `http://localhost:3000`

#### Frontend Configuration

The frontend loads its configuration from `web/config.json`:

```json
{
  "api_base_url": "http://localhost:8080"
}
```

For production deployment, ensure this file contains the correct API base URL for your environment.

## Project Structure

```
smart-todo/
├── cmd/
│   ├── server/              # API Backend entry point
│   │   └── main.go
│   └── configure/           # CLI Configuration Tool
│       ├── main.go
│       └── commands/
│           ├── oidc.go      # OIDC configuration commands
│           ├── list.go      # List configured providers
│           └── test.go      # Test OIDC configuration
├── internal/                # Private application code
│   ├── config/              # Configuration management
│   ├── database/            # Database layer
│   │   ├── db.go
│   │   ├── migrations/      # Database migrations
│   │   ├── users.go
│   │   ├── todos.go
│   │   └── oidc_config.go
│   ├── models/              # Data models
│   │   ├── user.go
│   │   ├── todo.go
│   │   ├── metadata.go
│   │   ├── oidc_config.go
│   │   └── jwt.go
│   ├── handlers/            # HTTP request handlers
│   │   ├── auth.go
│   │   ├── todos.go
│   │   ├── health.go
│   │   ├── openapi.go
│   │   └── helpers.go
│   ├── middleware/          # HTTP middleware
│   │   ├── auth.go          # JWT authentication
│   │   ├── cors.go
│   │   ├── logging.go
│   │   └── error.go
│   └── services/            # Business logic services
│       └── oidc/            # OIDC service
│           ├── provider.go
│           ├── jwks.go
│           ├── verifier.go
│           └── client.go
├── api/
│   └── openapi/
│       └── openapi.yaml     # OpenAPI 3.0 specification
├── web/                     # Frontend (Static PWA)
│   ├── index.html           # Login page
│   ├── app.html             # Main todo app
│   ├── config.json          # Frontend configuration
│   ├── manifest.json        # PWA manifest
│   ├── css/
│   │   └── style.css
│   └── js/
│       ├── config.js        # Configuration loader
│       ├── jwt.js           # JWT utilities
│       ├── api.js           # API client
│       ├── auth.js          # OIDC authentication
│       └── app.js           # Main application logic
└── docs/
    └── API.md               # API documentation
```

## API Endpoints

### Public Endpoints

- `GET /healthz` - Health check (basic mode)
- `GET /healthz?mode=extended` - Health check with database connectivity check
- `GET /health` - Legacy health check endpoint
- `GET /version` - Version information
- `GET /api/v1/openapi.yaml` - OpenAPI specification (YAML)
- `GET /api/v1/openapi.json` - OpenAPI specification (JSON)
- `GET /api/v1/auth/oidc/login` - Get OIDC configuration for frontend

### Protected Endpoints (Require JWT)

- `GET /api/v1/auth/me` - Get current user info
- `GET /api/v1/todos` - List todos (filterable by `time_horizon` and `status`, supports pagination with `page` and `page_size` query params)
- `POST /api/v1/todos` - Create todo
- `GET /api/v1/todos/:id` - Get todo by ID
- `PATCH /api/v1/todos/:id` - Update todo
- `DELETE /api/v1/todos/:id` - Delete todo
- `POST /api/v1/todos/:id/complete` - Mark todo as completed

See [OpenAPI specification](api/openapi/openapi.yaml) for complete API documentation.

## Health Checks

The `/healthz` endpoint provides health check functionality:

### Basic Mode (Default)

```bash
curl http://localhost:8080/healthz
```

Returns:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Extended Mode

```bash
curl http://localhost:8080/healthz?mode=extended
```

Returns:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "checks": {
    "database": "healthy"
  }
}
```

Extended mode checks:
- Database connectivity (5-second timeout)
- Future checks can be added (queueing, cache, etc.)

## Development Workflow

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests with race detection
go test -race ./...
```

### Code Quality Checks

```bash
# Run linter
make lint

# Run security scanner
make security

# Run vulnerability check
make vulnerability-check

# Run all quality checks
make all
```

### Building

```bash
# Build server
go build -o bin/smart-todo-server ./cmd/server

# Build configure tool
go build -o bin/smart-todo-configure ./cmd/configure

# Build for multiple platforms
make build
```

### Database Migrations

```bash
# Create a new migration
migrate create -ext sql -dir internal/database/migrations -seq migration_name

# Apply migrations
migrate -path internal/database/migrations -database "$DATABASE_URL" up

# Rollback one migration
migrate -path internal/database/migrations -database "$DATABASE_URL" down 1

# Check migration version
migrate -path internal/database/migrations -database "$DATABASE_URL" version
```

## Configuration

### Backend Configuration (Environment Variables)

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `DATABASE_URL` | PostgreSQL connection string | - | Yes |
| `REDIS_URL` | Redis connection URL for rate limiting | `redis://localhost:6379/0` | No (has default) |
| `SERVER_PORT` | Server port | `8080` | No |
| `BASE_URL` | Base URL for the API | `http://localhost:8080` | No |
| `FRONTEND_URL` | Frontend URL for CORS | `http://localhost:3000` | No |
| `OPENAI_API_KEY` | OpenAI API key (Phase 2) | - | No |
| `ENABLE_HSTS` | Enable HSTS header (production only, requires HTTPS) | `false` | No |
| `OIDC_PROVIDER` | OIDC provider name to use | `cognito` | No |
| `DEBUG` | Enable debug logging (includes verbose CORS logs) | `false` | No |

**Note**: Redis is required for rate limiting. The server will fail to start if Redis is unavailable. Redis connection URL format:
- `redis://localhost:6379/0` (local, no password)
- `redis://:password@host:6379/0` (with password)
- `redis://user:password@host:6379/0` (with username and password)

### Frontend Configuration (`web/config.json`)

```json
{
  "api_base_url": "http://localhost:8080"
}
```

This file should be deployed with the correct API URL for each environment.

### OIDC Configuration

OIDC configuration is stored in the database and managed via the CLI tool. The provider name can be any identifier (e.g., `cognito`, `okta`, `auth0`):

```bash
# For public clients (SPAs) - no client secret required
./bin/smart-todo-configure oidc cognito \
  --issuer "<cognito-issuer-url>" \
  --client-id "<client-id>" \
  --redirect-uri "<redirect-uri>"

# For confidential clients - with client secret
./bin/smart-todo-configure oidc cognito \
  --issuer "<cognito-issuer-url>" \
  --client-id "<client-id>" \
  --client-secret "<client-secret>" \
  --redirect-uri "<redirect-uri>"

# Example: Configure a different provider
./bin/smart-todo-configure oidc okta \
  --issuer "<okta-issuer-url>" \
  --client-id "<client-id>" \
  --redirect-uri "<redirect-uri>"
```

**Note**: 
- For frontend SPAs using Cognito public clients, the `--client-secret` flag is optional and should be omitted. Public clients are recommended for browser-based applications.
- The provider name used with `oidc <provider-name>` should match the `OIDC_PROVIDER` environment variable (defaults to `cognito`).
- Use the `list` command to see all configured providers: `./bin/smart-todo-configure list`

## Authentication Flow

1. User clicks "Sign in" on frontend
2. Frontend calls `GET /api/v1/auth/oidc/login` to get OIDC configuration
3. Frontend redirects user to Cognito authorization endpoint
4. User authenticates with Cognito
5. Cognito redirects back to frontend with authorization code
6. Frontend exchanges code for ID token (JWT)
7. Frontend stores JWT and includes it in `Authorization: Bearer <token>` header
8. Backend validates JWT using Cognito JWKS on each request

## API Versioning

All API endpoints use the `/api/v1/` prefix. Future versions will use `/api/v2/`, `/api/v3/`, etc.

Each API version maintains its own OpenAPI specification:
- `/api/v1/openapi.yaml`
- `/api/v2/openapi.yaml` (future)

## Security Features

The API server includes comprehensive security measures for public-facing deployment:

### Security Headers

All responses include security headers:
- `X-Content-Type-Options: nosniff` - Prevents MIME type sniffing
- `X-Frame-Options: DENY` - Prevents clickjacking
- `X-XSS-Protection: 1; mode=block` - Enables browser XSS filter
- `Referrer-Policy: strict-origin-when-cross-origin` - Controls referrer information
- `Permissions-Policy` - Restricts browser features (camera, microphone, geolocation disabled)
- `Content-Security-Policy: default-src 'none'` - Restrictive CSP for API endpoints
- `Strict-Transport-Security` (HSTS) - Only set when `ENABLE_HSTS=true` AND request is over HTTPS

**HSTS Safety for Local Development**: HSTS is **never** set for HTTP connections, even if `ENABLE_HSTS=true` is set. This prevents certificate issues in local development. In production behind an HTTPS proxy, explicitly set `ENABLE_HSTS=true` to enable HSTS.

### Rate Limiting

- **Backend**: Redis (required) - provides distributed rate limiting across multiple server instances
- **Unauthenticated endpoints**: 100 requests per minute per IP
- **Authenticated endpoints**: 1000 requests per minute per IP
- **Algorithm**: Sliding window counter for accurate rate limiting
- Rate limit headers are included in responses: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`
- Health check endpoints are exempt from rate limiting
- **Fail-open behavior**: If Redis is unavailable, requests are allowed (logged) to maintain availability

### Input Validation

- All user input is validated using struct tags and custom validators
- Todo text: 1-10,000 characters (after sanitization)
- Enum values (TimeHorizon, TodoStatus) are validated before database operations
- Text input is sanitized (whitespace trimmed, control characters removed)
- UUIDs are validated before use

### Request Size Limits

- Maximum request body size: 1MB (configurable via middleware)
- Maximum header size: 1MB (server-level configuration)
- JWKS response size limit: 10KB
- JWT token size limit: 8KB

### Request Timeouts

- Default request timeout: 30 seconds
- All database operations use context timeouts
- Prevents resource exhaustion from hanging requests

### Error Handling

- Error messages are sanitized before sending to clients
- Internal error details are logged server-side only
- Generic error messages prevent information disclosure

### Audit Logging

Security events are logged for monitoring:
- Failed authentication attempts (401)
- Authorization failures (403)
- Rate limit violations (429)
- Panic recovery events

### Path Traversal Protection

- File paths are validated to prevent directory traversal attacks
- OpenAPI spec handler validates paths are within allowed directory

### JWT Token Security

- Token length validation (max 8KB)
- JWKS URL validation (must be HTTPS)
- Token expiration and signature verification
- JWKS response size limits

## Deployment

### Backend Deployment

The backend can be deployed as:
- Go binary on a server
- Docker container
- Kubernetes deployment
- Any container orchestration platform

Ensure environment variables are set correctly and database migrations have been run.

### Production Deployment Security Checklist

Before deploying to production:

- [ ] Set `ENABLE_HSTS=true` when behind HTTPS proxy
- [ ] Verify all security headers are present in responses
- [ ] Review rate limiting settings for your use case
- [ ] Ensure `OIDC_PROVIDER` matches your configured provider name
- [ ] Set `DEBUG=false` or omit it (defaults to false) to reduce logging
- [ ] Verify request size limits are appropriate
- [ ] Test error responses don't leak internal details
- [ ] Ensure health checks are not publicly accessible if sensitive

### Frontend Deployment

The frontend is a static Progressive Web App that can be deployed to:
- Netlify
- Vercel
- AWS S3 + CloudFront
- GitHub Pages
- Any static file hosting service

**Important**: Deploy `web/config.json` with the correct `api_base_url` for your environment.

### Docker Deployment

```bash
# Build backend Docker image
docker build -f server.Dockerfile -t smart-todo-server .

# Run backend container
docker run -p 8080:8080 \
  -e DATABASE_URL="postgres://..." \
  -e SERVER_PORT="8080" \
  smart-todo-server
```

## Troubleshooting

### Database Connection Issues

```bash
# Test database connection
psql "$DATABASE_URL" -c "SELECT 1;"

# Check if migrations have been run
migrate -path internal/database/migrations -database "$DATABASE_URL" version
```

### OIDC Configuration Issues

```bash
# List configured providers
./bin/smart-todo-configure list

# Test OIDC configuration (use the provider name from OIDC_PROVIDER env var)
./bin/smart-todo-configure test --provider cognito

# Check OIDC config in database
psql "$DATABASE_URL" -c "SELECT provider, issuer FROM oidc_config;"
```

### Frontend API Connection Issues

1. Check `web/config.json` has correct `api_base_url`
2. Verify backend is running and accessible
3. Check browser console for CORS errors
4. Verify `FRONTEND_URL` environment variable matches frontend URL

### Security-Related Issues

#### Rate Limiting

If requests are being rate limited (HTTP 429):
- Check `X-RateLimit-Remaining` header to see remaining requests
- Authenticated endpoints have higher limits (1000 req/min) than unauthenticated (100 req/min)
- Wait for the rate limit window to reset or implement exponential backoff in your client

#### Request Size Limits

If receiving "Request Entity Too Large" (HTTP 413):
- Default limit is 1MB for request bodies
- Reduce payload size or split large requests into multiple smaller requests

#### HSTS Issues in Local Development

HSTS should **never** cause issues in local development because:
- HSTS is only set when both `ENABLE_HSTS=true` AND the request is over HTTPS
- Local development typically uses HTTP (localhost), so HSTS is never set
- If you're testing with HTTPS locally and HSTS is set, you'll need to clear your browser's HSTS cache

#### Input Validation Errors

If receiving validation errors:
- Check that todo text is between 1-10,000 characters
- Verify enum values (`time_horizon`: `now`, `soon`, `later`; `status`: `pending`, `processing`, `completed`)
- Ensure required fields are provided
- Check that UUIDs are in valid format

#### Debug Logging

To enable verbose logging (including CORS debug logs):
```bash
export DEBUG=true
```

This will output detailed CORS request logging which can help diagnose CORS issues during development.

## Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
