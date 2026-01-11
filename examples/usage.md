# Usage Examples

This document provides examples of how to use the AI Code Template Go in various scenarios.

## Quick Start

### 1. Clone and Setup

```bash
# Clone the template
git clone https://github.com/benvon/ai-code-template-go.git my-project
cd my-project

# Run the setup script
./scripts/setup.sh

# Or manually install dependencies
go mod tidy
```

### 2. Run the Application

```bash
# Run directly with Go
go run cmd/server/main.go

# Or use the Makefile
make build
./bin/ai-code-template-go

# Or run with Docker
make docker-build
make docker-run
```

### 3. Test the API

```bash
# Health check
curl http://localhost:8080/health

# Version info
curl http://localhost:8080/version

# Hello endpoint
curl http://localhost:8080/api/v1/hello

# Status endpoint
curl http://localhost:8080/api/v1/status
```

## Development Workflow

### 1. Running Tests

```bash
# Run all tests
make test

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run benchmarks
make benchmark
```

### 2. Code Quality Checks

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

### 3. Building

```bash
# Build for current platform
make build

# Build for specific platform
GOOS=linux GOARCH=amd64 go build -o bin/app-linux-amd64 ./cmd/server

# Build Docker image
make docker-build
```

## Docker Development

### 1. Using Docker Compose

```bash
# Start services
make docker-compose-up

# View logs
docker-compose logs -f app

# Stop services
make docker-compose-down
```

### 2. Manual Docker Commands

```bash
# Build image
docker build -t my-app .

# Run container
docker run -p 8080:8080 my-app

# Run with environment variables
docker run -p 8080:8080 -e APP_PORT=9090 my-app
```

## Configuration

### 1. Environment Variables

Create a `.env` file based on `.env.example`:

```bash
# Copy example configuration
cp .env.example .env

# Edit configuration
nano .env
```

### 2. Configuration Structure

The application uses a hierarchical configuration system:

```go
type Config struct {
    Server   ServerConfig
    Database DatabaseConfig
    Redis    RedisConfig
    Logging  LoggingConfig
    Security SecurityConfig
}
```

## Adding New Features

### 1. Adding New Endpoints

```go
// In internal/handlers/handlers.go
func RegisterRoutes(mux *http.ServeMux) {
    // Existing routes...
    mux.HandleFunc("/api/v1/users", handleUsers)
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
    // Implementation...
}
```

### 2. Adding New Configuration

```go
// In internal/config/config.go
type Config struct {
    // Existing config...
    Email EmailConfig
}

type EmailConfig struct {
    Host     string
    Port     int
    Username string
    Password string
}
```

### 3. Adding New Models

```go
// In internal/models/user.go
package models

type User struct {
    ID       string    `json:"id"`
    Email    string    `json:"email"`
    Name     string    `json:"name"`
    Created  time.Time `json:"created"`
    Updated  time.Time `json:"updated"`
}
```

## Testing Patterns

### 1. Table-Driven Tests

```go
func TestHandleHello(t *testing.T) {
    tests := []struct {
        name           string
        method         string
        expectedStatus int
        expectedBody   string
    }{
        {"GET request", "GET", http.StatusOK, "Hello from AI Code Template Go!"},
        {"POST request", "POST", http.StatusMethodNotAllowed, ""},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation...
        })
    }
}
```

### 2. Mocking Dependencies

```go
type UserService interface {
    GetUser(id string) (*models.User, error)
}

type MockUserService struct {
    users map[string]*models.User
}

func (m *MockUserService) GetUser(id string) (*models.User, error) {
    if user, exists := m.users[id]; exists {
        return user, nil
    }
    return nil, errors.New("user not found")
}
```

## Deployment

### 1. Local Development

```bash
# Run with hot reload (if using air)
go install github.com/cosmtrek/air@latest
air

# Or use the Makefile
make docker-compose-up
```

### 2. Production Build

```bash
# Build optimized binary
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-s -w" -o main ./cmd/server

# Build Docker image
docker build -t my-app:latest .
```

### 3. Using GoReleaser

```bash
# Create and push a tag
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0

# GoReleaser will automatically:
# - Build binaries for multiple platforms
# - Create GitHub release
# - Generate checksums
# - Upload artifacts
```

## Troubleshooting

### 1. Common Issues

```bash
# Port already in use
lsof -i :8080
kill -9 <PID>

# Permission denied
chmod +x scripts/setup.sh

# Go module issues
go mod tidy
go mod download
```

### 2. Debug Mode

```bash
# Enable debug logging
export LOG_LEVEL=debug
export APP_DEBUG=true

# Run with verbose output
go run -v ./cmd/server/main.go
```

### 3. Health Checks

```bash
# Check application health
curl http://localhost:8080/health

# Check Docker container health
docker ps
docker inspect <container_id> | grep Health -A 10
```

## Best Practices

### 1. Code Organization

- Keep packages focused and cohesive
- Use interfaces for dependency injection
- Separate business logic from infrastructure
- Follow Go naming conventions

### 2. Error Handling

- Always check error return values
- Wrap errors with context
- Use custom error types when appropriate
- Log errors with sufficient detail

### 3. Testing

- Write tests for all exported functions
- Use table-driven tests for multiple scenarios
- Mock external dependencies
- Aim for >80% test coverage

### 4. Security

- Validate all input data
- Use HTTPS in production
- Implement proper authentication
- Follow OWASP guidelines
