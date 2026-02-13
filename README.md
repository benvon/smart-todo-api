# Smart Todo - LLM-Integrated Todo List Manager

A simple yet powerful todo list manager with a Go REST API backend, PostgreSQL database, static web frontend, and OpenAI integration for smart categorization and metadata tagging.

## Overview

Smart Todo is designed to be simple and intuitive, allowing users to quickly input todo items while the backend handles categorization and metadata using AI. The application uses OIDC authentication (currently supporting AWS Cognito) and provides a clean three-column interface organizing todos by time horizon: Next, Soon, and Later.

### Key Features

- **Automatic Task Analysis**: Tasks are automatically analyzed by AI to extract category tags and assign time horizons
- **Interactive AI Chat**: Users can chat with the AI to provide context and preferences for task categorization
- **Tag Management**: AI-generated tags with user override capability - user-defined tags always take precedence
- **Smart Reprocessing**: Automatic re-analysis of tasks (2x daily) to update time horizons as priorities change
- **Activity-Based Pausing**: Reprocessing pauses after 3 days of inactivity and resumes on user login

### Architecture

- **Backend**: Go REST API server with PostgreSQL database, API versioning (`/api/v1/`), and OpenAPI contract
- **Frontend**: Static Progressive Web App (PWA) that can be built and deployed independently
- **Authentication**: OIDC-based authentication using JWT tokens (AWS Cognito first)
- **Queue System**: RabbitMQ for asynchronous job processing (AI analysis, reprocessing)
- **Rate Limiting**: Redis-based distributed rate limiting
- **CLI Tool**: Configuration tool for setting up OIDC/Cognito authentication

---

## Quick Start (For Operators)

### Prerequisites

- **PostgreSQL 17+** - [Install PostgreSQL](https://www.postgresql.org/download/)
  - Includes the `createdb` command-line utility for creating databases
- **Redis 7+** - [Install Redis](https://redis.io/download) - Required for rate limiting
- **RabbitMQ 3.12+** - [Install RabbitMQ](https://www.rabbitmq.com/download.html) - Required for job queueing (with delayed message exchange plugin)
- **golang-migrate** - Database migration tool for managing schema changes
  - [Installation instructions](https://github.com/golang-migrate/migrate/blob/master/cmd/migrate/README.md)
  - macOS: `brew install golang-migrate`
  - Linux: Download from [releases](https://github.com/golang-migrate/migrate/releases)
  - Provides the `migrate` command-line tool
- **AWS Cognito** (or other OIDC provider) - For authentication
- **OpenAI API Key** - For AI features (optional, but required for AI functionality)

### Docker Compose (Recommended for Local Development)

The fastest way to get started is using Docker Compose:

```bash
# Copy the example .env file
cp .env.example .env

# Edit .env and set your OpenAI API key
# OPENAI_API_KEY=your-actual-api-key-here

# Start all services (PostgreSQL, Redis, RabbitMQ, Server, Worker, Web)
docker-compose up -d

# View logs
docker-compose logs -f

# Stop all services
docker-compose down
```

**Note**: The `.env` file is gitignored and contains sensitive information. Make sure to never commit it to version control.

### Manual Setup

1. **Set Up Database**

   ```bash
   # Create the database using PostgreSQL's createdb utility
   # (createdb comes with PostgreSQL installation)
   createdb smarttodo
   
   # Set the database connection URL
   export DATABASE_URL="postgres://user:password@localhost/smarttodo?sslmode=disable"
   
   # Run database migrations using golang-migrate
   # (migrate is the command-line tool from golang-migrate)
   migrate -path internal/database/migrations -database "$DATABASE_URL" up
   ```

2. **Set Up RabbitMQ**

   ```bash
   # Install and start RabbitMQ (macOS)
   brew install rabbitmq
   brew services start rabbitmq
   rabbitmq-plugins enable rabbitmq_delayed_message_exchange
   ```

3. **Configure Environment Variables**

   ```bash
   export DATABASE_URL="postgres://user:password@localhost/smarttodo?sslmode=disable"
   export REDIS_URL="redis://localhost:6379/0"
   export RABBITMQ_URL="amqp://guest:guest@localhost:5672/"
   export SERVER_PORT="8080"
   export BASE_URL="http://localhost:8080"
   export FRONTEND_URL="http://localhost:3000"
   export OPENAI_API_KEY="your-api-key-here"  # Required for AI features
   ```

4. **Configure OIDC Authentication**

   ```bash
   # Build the configure tool
   go build -o bin/smart-todo-configure ./cmd/configure
   
   # Configure AWS Cognito (public client - no secret required)
   ./bin/smart-todo-configure oidc cognito \
     --issuer "https://cognito-idp.<region>.amazonaws.com/<pool-id>" \
     --client-id "<your-client-id>" \
     --redirect-uri "http://localhost:3000/index.html"
   
   # List configured providers
   ./bin/smart-todo-configure list
   ```

5. **Run the Application**

   ```bash
   # Terminal 1: Start the API server
   go run ./cmd/server
   
   # Terminal 2: Start the worker (required for AI features)
   go run ./cmd/worker
   
   # Terminal 3: Build and serve the frontend
   cd web
   cp config.json.example config.json
   # Edit config.json to set api_base_url
   npm install
   npm run build
   # Serve the built files (or use any static file server)
   python3 -m http.server 3000
   ```

The API will be available at `http://localhost:8080` and the frontend at `http://localhost:3000`.

---

## For Developers

### Development Setup

#### Developer Prerequisites

- **Go 1.25+** - [Install Go](https://go.dev/doc/install)
- **Node.js 18+** - [Install Node.js](https://nodejs.org/) - Required for frontend build system

#### Getting Started

```bash
# Clone the repository
git clone <repository-url> smart-todo
cd smart-todo

# Install dependencies
go mod tidy

# Follow the Quick Start section above to set up services
```

### Project Structure

```text
smart-todo/
├── cmd/
│   ├── server/              # API Backend entry point
│   ├── worker/              # Worker process entry point
│   └── configure/           # CLI Configuration Tool
├── internal/                # Private application code
│   ├── config/              # Configuration management
│   ├── database/            # Database layer and migrations
│   ├── models/              # Data models
│   ├── handlers/            # HTTP request handlers
│   ├── middleware/          # HTTP middleware
│   ├── services/            # Business logic services
│   │   ├── oidc/            # OIDC service
│   │   └── ai/              # AI service
│   ├── queue/               # Job queue system
│   └── workers/             # Worker processes
├── api/
│   └── openapi/
│       └── openapi.yaml     # OpenAPI 3.0 specification
├── web/                     # Frontend (Static PWA)
└── docs/                    # Additional documentation
    ├── API.md               # API documentation
    ├── DATA_MODEL.md        # Data model, isolation, and migrations
    ├── TESTING.md           # Testing guide
    └── QUEUE_SCALING.md     # Queue scaling guide
```

### Development Workflow

#### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests with race detection
go test -race ./...
```

#### Code Quality Checks

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

#### Building

**Backend:**

```bash
# Build server
go build -o bin/smart-todo-server ./cmd/server

# Build worker
go build -o bin/smart-todo-worker ./cmd/worker

# Build configure tool
go build -o bin/smart-todo-configure ./cmd/configure

# Build for multiple platforms
make build
```

**Frontend:**

```bash
# Navigate to web directory
cd web

# Install dependencies (first time only)
npm install

# Build for production
npm run build

# Build in watch mode for development (auto-rebuilds on changes)
npm run dev

# Run tests
npm test

# Run tests with coverage
npm run test:coverage

# Lint code
npm run lint

# Fix linting issues automatically
npm run lint:fix

# Run security audit
npm run security:audit

# Check for security issues and outdated packages
npm run security:check
```

The frontend build system:
- Bundles all JavaScript files using esbuild
- Outputs to `web/dist/` directory
- Creates separate bundles: `dist/app.js` (for app.html) and `dist/index.js` (for index.html)
- Generates source maps for debugging
- Uses chrono-node for natural language date parsing
- Uses dayjs for date formatting and manipulation

**Frontend Quality Tools:**
- **ESLint** - Code linting and style enforcement
- **c8** - Test coverage reporting
- **npm audit** - Security vulnerability scanning
- All tools are integrated into the Makefile and CI pipeline

#### Database Migrations

The project uses [golang-migrate](https://github.com/golang-migrate/migrate) for database schema management. For table descriptions, user scoping (data isolation), and tag-statistics derivation, see [docs/DATA_MODEL.md](docs/DATA_MODEL.md). The `migrate` command-line tool must be installed (see Prerequisites section).

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

### Key Dependencies

- **github.com/gorilla/mux** - HTTP router
- **github.com/lib/pq** - PostgreSQL driver
- **github.com/redis/go-redis/v9** - Redis client for distributed rate limiting
- **github.com/rabbitmq/amqp091-go** - RabbitMQ client for job queueing
- **github.com/lestrrat-go/jwx/v2** - JWT token verification
- **github.com/go-playground/validator/v10** - Input validation library
- **github.com/openai/openai-go/v3** - OpenAI API client

See `go.mod` for the complete dependency list.

### Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

---

## For Operators

### Configuration

#### Environment Variables

| Variable | Description | Default | Required |
| --- | --- | --- | --- |
| `DATABASE_URL` | PostgreSQL connection string | - | Yes |
| `REDIS_URL` | Redis connection URL for rate limiting | `redis://localhost:6379/0` | No |
| `RABBITMQ_URL` | RabbitMQ connection URL for job queueing | - | Yes |
| `SERVER_PORT` | Server port | `8080` | No |
| `BASE_URL` | Base URL for the API | `http://localhost:8080` | No |
| `FRONTEND_URL` | Frontend URL for CORS | `http://localhost:3000` | No |
| `OPENAI_API_KEY` | OpenAI API key | - | No (required for AI features) |
| `AI_PROVIDER` | AI provider to use | `openai` | No |
| `AI_MODEL` | AI model to use | `gpt-5-mini` | No |
| `AI_BASE_URL` | AI API base URL (for custom endpoints) | - | No |
| `ENABLE_HSTS` | Enable HSTS header (production only, requires HTTPS) | `false` | No |
| `OIDC_PROVIDER` | OIDC provider name to use | `cognito` | No |
| `RABBITMQ_PREFETCH` | Number of unacknowledged messages per worker | `1` | No |
| `DEBUG` | Enable debug logging | `false` | No |

**Connection URL Formats:**

- **Redis**: `redis://localhost:6379/0` or `redis://:password@host:6379/0` or `redis://user:password@host:6379/0`
- **RabbitMQ**: `amqp://guest:guest@localhost:5672/` or `amqp://user:password@host:5672/vhost`

**Notes:**

- Redis is required for rate limiting. The server will fail to start if Redis is unavailable.
- RabbitMQ is required for job queueing. The server will fail to start if RabbitMQ is unavailable.
- AI features require both `OPENAI_API_KEY` and `RABBITMQ_URL`. The server will start without `OPENAI_API_KEY`, but AI features will be disabled.

#### Frontend Configuration

The frontend loads its configuration from `web/config.json`:

```json
{
  "api_base_url": "http://localhost:8080"
}
```

This file should be deployed with the correct API URL for each environment.

**Frontend Build System:**

The frontend uses a modern build system with:
- **esbuild** - Fast JavaScript bundler
- **chrono-node** - Natural language date parsing
- **dayjs** - Lightweight date manipulation library

Build configuration is in `web/esbuild.config.js`. The build system:
- Bundles all JavaScript files into `web/dist/`
- Generates source maps for debugging
- Supports watch mode for development (`npm run dev`)
- Creates separate bundles for login page (`index.js`) and app page (`app.js`)

All build artifacts are in `web/dist/` and should be included in deployments.

#### OIDC Configuration

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
```

**Note**: The provider name used with `oidc <provider-name>` should match the `OIDC_PROVIDER` environment variable (defaults to `cognito`).

### Deployment

#### Production Deployment Security Checklist

Before deploying to production:

- [ ] Set `ENABLE_HSTS=true` when behind HTTPS proxy
- [ ] Verify all security headers are present in responses
- [ ] Review rate limiting settings for your use case
- [ ] Ensure `OIDC_PROVIDER` matches your configured provider name
- [ ] Set `DEBUG=false` or omit it (defaults to false) to reduce logging
- [ ] Verify request size limits are appropriate
- [ ] Test error responses don't leak internal details
- [ ] Ensure health checks are not publicly accessible if sensitive

#### Docker Deployment

```bash
# Build backend Docker image
docker build -f server.Dockerfile -t smart-todo-server .

# Build worker Docker image
docker build -f worker.Dockerfile -t smart-todo-worker .

# Run backend container
docker run -p 8080:8080 \
  -e DATABASE_URL="postgres://..." \
  -e REDIS_URL="redis://..." \
  -e RABBITMQ_URL="amqp://..." \
  -e OPENAI_API_KEY="..." \
  -e SERVER_PORT="8080" \
  smart-todo-server

# Run worker container (separate process)
docker run \
  -e DATABASE_URL="postgres://..." \
  -e REDIS_URL="redis://..." \
  -e RABBITMQ_URL="amqp://..." \
  -e OPENAI_API_KEY="..." \
  smart-todo-worker
```

**Note**: The worker should run as a separate container/process. Multiple worker instances can run for horizontal scaling. See [docs/QUEUE_SCALING.md](docs/QUEUE_SCALING.md) for scaling guidance.

#### Frontend Deployment

On version tags (`*.*.*`), the frontend is built, deployed to Cloudflare Pages (when secrets are configured), and a release tarball is published. See [docs/DEPLOYING_FRONTEND.md](docs/DEPLOYING_FRONTEND.md) for automated Cloudflare Pages deployment and required GitHub Actions secrets (`WEB_API_BASE_URL`, `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID`, `CLOUDFLARE_PAGES_PROJECT_NAME`).

The frontend is a static Progressive Web App that can be deployed to:

- Cloudflare Pages
- Netlify
- Vercel
- AWS S3 + CloudFront
- GitHub Pages
- Any static file hosting service

**Building the Frontend:**

The frontend uses a build system with esbuild for bundling JavaScript files. To build the frontend:

```bash
# Navigate to the web directory
cd web

# Install dependencies (first time only)
npm install

# Build for production
npm run build

# Or run in watch mode for development
npm run dev
```

The build output will be in `web/dist/`:
- `dist/app.js` - Main application bundle (for app.html)
- `dist/index.js` - Login page bundle (for index.html)

**Deployment Steps:**

1. Build the frontend: `cd web && npm install && npm run build`
2. Deploy the contents of the `web/` directory, including:
   - `dist/` - Built JavaScript bundles
   - `index.html` and `app.html` - HTML files (updated to use bundled JS)
   - `css/` - Stylesheets
   - `config.json` - API configuration (with correct `api_base_url`)
   - `manifest.json` - PWA manifest

**Important**: 
- Deploy `web/config.json` with the correct `api_base_url` for your environment
- Ensure `dist/app.js` and `dist/index.js` are included in deployment
- The build system is self-contained within the `web/` directory

#### Kubernetes Deployment

Kubernetes manifests are available in the `k8s/` directory. See the individual files for deployment instructions.

### Health Checks

The `/healthz` endpoint provides health check functionality:

**Basic Mode (Default):**

```bash
curl http://localhost:8080/healthz
```

**Extended Mode:**

```bash
curl http://localhost:8080/healthz?mode=extended
```

Extended mode checks database connectivity (5-second timeout). Health check endpoints are exempt from rate limiting.

### Troubleshooting

#### Database Connection Issues

```bash
# Test database connection using psql (PostgreSQL command-line client)
psql "$DATABASE_URL" -c "SELECT 1;"

# Check if migrations have been run (requires golang-migrate)
migrate -path internal/database/migrations -database "$DATABASE_URL" version
```

#### OIDC Configuration Issues

```bash
# List configured providers
./bin/smart-todo-configure list

# Test OIDC configuration
./bin/smart-todo-configure test --provider cognito

# Check OIDC config in database
psql "$DATABASE_URL" -c "SELECT provider, issuer FROM oidc_config;"
```

#### Frontend API Connection Issues

1. Check `web/config.json` has correct `api_base_url`
2. Verify backend is running and accessible
3. Check browser console for CORS errors
4. Verify `FRONTEND_URL` environment variable matches frontend URL
5. Ensure frontend is built: `cd web && npm install && npm run build`
6. Verify `dist/app.js` and `dist/index.js` exist and are being served

#### AI Features Not Working

- Verify `OPENAI_API_KEY` is set correctly
- Check that `RABBITMQ_URL` is configured and RabbitMQ is running
- Ensure the RabbitMQ delayed message exchange plugin is enabled: `rabbitmq-plugins enable rabbitmq_delayed_message_exchange`
- Verify the worker process is running (required for processing AI analysis jobs)
- Check worker logs for errors processing jobs

#### RabbitMQ Connection Issues

- Verify RabbitMQ is running: `rabbitmqctl status`
- Check connection URL format: `amqp://user:password@host:5672/vhost`
- Ensure delayed message exchange plugin is enabled
- For Docker: verify RabbitMQ service is healthy in `docker-compose ps`

#### Rate Limiting

If requests are being rate limited (HTTP 429):

- Check `X-RateLimit-Remaining` header to see remaining requests
- Authenticated endpoints have higher limits (1000 req/min) than unauthenticated (100 req/min)
- Wait for the rate limit window to reset or implement exponential backoff in your client

#### Debug Logging

To enable verbose logging (including CORS debug logs):

```bash
export DEBUG=true
```

---

## Reference

### API Endpoints

#### Public Endpoints

- `GET /healthz` - Health check (basic mode)
- `GET /healthz?mode=extended` - Health check with database connectivity check
- `GET /health` - Legacy health check endpoint
- `GET /version` - Version information
- `GET /api/v1/openapi.yaml` - OpenAPI specification (YAML)
- `GET /api/v1/openapi.json` - OpenAPI specification (JSON)
- `GET /api/v1/auth/oidc/login` - Get OIDC configuration for frontend

#### Protected Endpoints (Require JWT)

- `GET /api/v1/auth/me` - Get current user info
- `GET /api/v1/todos` - List todos (filterable by `time_horizon` and `status`, supports pagination)
- `POST /api/v1/todos` - Create todo (automatically queues AI analysis job)
- `GET /api/v1/todos/:id` - Get todo by ID
- `PATCH /api/v1/todos/:id` - Update todo (supports tag management)
- `DELETE /api/v1/todos/:id` - Delete todo
- `POST /api/v1/todos/:id/complete` - Mark todo as completed
- `POST /api/v1/todos/:id/analyze` - Manually trigger AI analysis (returns 202 Accepted)
- `GET /api/v1/ai/chat` - Start AI chat session (Server-Sent Events)
- `POST /api/v1/ai/chat` - Send message in AI chat session

**Notes:**

- Time horizon values: `next`, `soon`, `later`
- Status values: `pending`, `processing`, `processed`, `completed`
- AI chat uses Server-Sent Events (SSE) for real-time streaming responses

For complete API documentation, see:

- [OpenAPI specification](api/openapi/openapi.yaml)
- [API Documentation](docs/API.md)

### Authentication Flow

1. User clicks "Sign in" on frontend
2. Frontend calls `GET /api/v1/auth/oidc/login` to get OIDC configuration
3. Frontend redirects user to Cognito authorization endpoint
4. User authenticates with Cognito
5. Cognito redirects back to frontend with authorization code
6. Frontend exchanges code for ID token (JWT)
7. Frontend stores JWT and includes it in `Authorization: Bearer <token>` header
8. Backend validates JWT using Cognito JWKS on each request

### API Versioning

All API endpoints use the `/api/v1/` prefix. Future versions will use `/api/v2/`, `/api/v3/`, etc.

Each API version maintains its own OpenAPI specification:

- `/api/v1/openapi.yaml`
- `/api/v2/openapi.yaml` (future)

### Security Features

The API server includes comprehensive security measures:

- **Security Headers**: X-Content-Type-Options, X-Frame-Options, X-XSS-Protection, Referrer-Policy, Permissions-Policy, Content-Security-Policy, HSTS (when enabled and over HTTPS)
- **Rate Limiting**: Redis-based distributed rate limiting (100 req/min unauthenticated, 1000 req/min authenticated)
- **Input Validation**: All user input validated with struct tags and custom validators
- **Request Size Limits**: 1MB request body, 1MB headers, 8KB JWT tokens, 10KB JWKS responses
- **Request Timeouts**: 30-second default timeout, context timeouts for database operations
- **Error Handling**: Sanitized error messages, internal details logged server-side only
- **Audit Logging**: Security events logged (401, 403, 429, panics)
- **JWT Token Security**: Token length validation, JWKS URL validation (HTTPS required), expiration and signature verification

**HSTS Safety**: HSTS is **never** set for HTTP connections, even if `ENABLE_HSTS=true` is set. This prevents certificate issues in local development.

### AI Features

#### Automatic Task Analysis

When a new todo is created, an AI analysis job is automatically queued. The worker process:

1. Analyzes the task text using the configured AI provider (OpenAI)
2. Extracts category tags (e.g., "work", "personal", "urgent", "email")
3. Suggests a time horizon (`next`, `soon`, or `later`)
4. Merges AI-generated tags with any existing user-defined tags (user tags take precedence)

Analysis happens asynchronously, so the API returns immediately while processing continues in the background.

#### Interactive AI Chat

Users can chat with the AI to provide context and preferences:

- Start a chat session via `GET /api/v1/ai/chat` (Server-Sent Events)
- Send messages via `POST /api/v1/ai/chat`
- The AI uses conversation history to better categorize tasks
- Conversation summaries are stored and used in future task analysis

#### Tag Management

- **AI-Generated Tags**: Automatically extracted from task text
- **User-Defined Tags**: Users can add/remove tags via the API
- **Tag Override**: User-defined tags always take precedence over AI-generated tags
- **Tag Sources**: System tracks whether tags are from AI or user input

#### Reprocessing Schedule

Tasks are automatically re-analyzed:

- **Frequency**: Twice daily (morning and evening)
- **Pause Logic**: Reprocessing pauses after 3 days of user inactivity
- **Resume Logic**: Reprocessing resumes when user logs in again
- **Eligibility**: Only active users (not paused) receive reprocessing

---

## Additional Documentation

- [API Documentation](docs/API.md) - Detailed API endpoint documentation
- [Testing Guide](docs/TESTING.md) - Testing strategies and examples
- [Queue Scaling Guide](docs/QUEUE_SCALING.md) - Scaling worker processes
- [Wiring Checklist](docs/WIRING_CHECKLIST.md) - Setup verification checklist
- [Contributing Guide](CONTRIBUTING.md) - How to contribute to the project

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
