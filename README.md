# ai-code-template-go

A template repo that's useful for Go code to be built with the help of agentic AI.

## Features

- ✅ Comprehensive CI/CD pipeline with GitHub Actions
- ✅ Automated releases with GoReleaser
- ✅ Cross-platform builds (Linux, macOS, Windows on amd64 and arm64)
- ✅ Security scanning with Gosec and govulncheck
- ✅ Code quality checks with golangci-lint
- ✅ Dependency management with Dependabot
- ✅ Test coverage reporting
- ✅ Docker containerization with multi-stage builds
- ✅ Docker Compose for local development
- ✅ Pre-commit hooks for code quality
- ✅ VS Code configuration for Go development
- ✅ Comprehensive project structure examples
- ✅ Environment configuration management
- ✅ Setup automation scripts
- ✅ API documentation and examples

## Getting Started

1. **Clone and customize**:
   ```bash
   # Clone this template
   git clone https://github.com/benvon/ai-code-template-go.git your-project-name
   cd your-project-name
   
   # Update go.mod with your module path
   go mod edit -module github.com/your-username/your-project-name
   ```

2. **Install dependencies**:
   ```bash
   go mod tidy
   ```

3. **Run tests**:
   ```bash
   go test ./...
   ```

4. **Build locally**:
   ```bash
   go build -o ./bin/your-project-name ./
   ```

## Release Process

This project uses GoReleaser for automated releases. When you push a semantic version tag, it will:

1. Build binaries for multiple platforms (Linux, macOS, Windows on amd64 and arm64)
2. Create GitHub releases with changelogs
3. Generate checksums
4. Upload release artifacts

### Creating a Release

1. **Tag your release**:
   ```bash
   # Create and push a tag
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

2. **Supported tag formats**:
   - `v1.0.0` - Standard release
   - `v1.0.0-beta.1` - Pre-release
   - `v1.0.0-rc.1` - Release candidate

3. **The release workflow will**:
   - Run all tests
   - Build binaries for:
     - Linux (amd64, arm64)
     - macOS (amd64, arm64)
     - Windows (amd64, arm64)
   - Create GitHub release with:
     - Release notes
     - Binary downloads
     - Checksums
     - Changelog

### Binary Naming Convention

Released binaries follow this pattern:
```
ai-code-template-go_v1.0.0_Linux_x86_64.tar.gz
ai-code-template-go_v1.0.0_Darwin_arm64.tar.gz
ai-code-template-go_v1.0.0_Windows_x86_64.zip
```

### Version Information

Built binaries include version information accessible via:
```bash
./your-binary --version
```

This displays:
- Version number
- Git commit hash
- Build date
- Built by (goreleaser)

## Configuration Files

- `.goreleaser.yml` - GoReleaser configuration for builds and releases
- `.github/workflows/ci.yml` - CI pipeline (tests, linting, security)
- `.github/workflows/release.yml` - Release pipeline
- `.golangci.yml` - Linter configuration
- `.github/dependabot.yml` - Dependency update configuration
- `Dockerfile` - Multi-stage Docker build configuration
- `docker-compose.yml` - Local development environment
- `.pre-commit-config.yaml` - Pre-commit hooks configuration
- `.vscode/settings.json` - VS Code Go development settings
- `.vscode/extensions.json` - Recommended VS Code extensions
- `.env.example` - Environment variables template

## Development

This project includes several CI checks:
- **Tests**: Unit tests with race detection and coverage
- **Linting**: golangci-lint with multiple linters enabled
- **Security**: Gosec security scanner and govulncheck
- **Builds**: Cross-platform build verification
- **Dependencies**: Go mod tidy verification

All checks must pass before merging to main.

### Project Structure

```
ai-code-template-go/
├── cmd/                    # Application entry points
│   └── server/            # HTTP server application
├── internal/               # Private application code
│   ├── config/            # Configuration management
│   ├── handlers/          # HTTP request handlers
│   └── models/            # Data models
├── pkg/                    # Public libraries
├── api/                    # API definitions
├── docs/                   # Documentation
├── scripts/                # Build and deployment scripts
├── examples/               # Usage examples
└── .env.example           # Environment variables template
```

### Docker Development

```bash
# Build and run with Docker
make docker-build
make docker-run

# Or use Docker Compose for local development
make docker-compose-up
make docker-compose-down
```

### Pre-commit Hooks

This project includes pre-commit hooks for code quality:

```bash
# Install pre-commit hooks
pre-commit install

# Run manually
pre-commit run --all-files
```

### VS Code Integration

VS Code configuration is included for optimal Go development experience. The recommended extensions will be automatically suggested when opening the project.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.
