# Contributing to ai-code-template-go

Thank you for your interest in contributing to this AI-assisted Go template repository! This guide will help you get started.

## Code of Conduct

By participating in this project, you agree to abide by our Code of Conduct. Please be respectful and inclusive in all interactions.

## How Can I Contribute?

### Reporting Bugs

- Use the bug report template when creating issues
- Include detailed steps to reproduce
- Provide your Go version, OS, and platform information
- Include relevant log output

### Suggesting Features

- Use the feature request template
- Clearly describe the use case
- Explain how it would benefit users of this template

### Submitting Pull Requests

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Add tests for your changes
5. Run the test suite (`go test ./...`)
6. Run linting (`golangci-lint run`)
7. Commit your changes (`git commit -m 'Add amazing feature'`)
8. Push to your branch (`git push origin feature/amazing-feature`)
9. Open a Pull Request using the PR template

## Development Setup

### Prerequisites

- Go 1.21 or later
- Git
- golangci-lint (for linting)
- GoReleaser (for testing releases)

### Local Development

```bash
# Clone your fork
git clone https://github.com/YOUR-USERNAME/ai-code-template-go.git
cd ai-code-template-go

# Install dependencies
go mod tidy

# Run tests
go test ./...

# Run linter
golangci-lint run

# Build locally
go build -o ./bin/app ./
```

## Coding Standards

### Go Style Guide

- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` to format your code
- Run `golangci-lint run` before submitting
- Write meaningful commit messages

### Testing

- Write unit tests for new functionality
- Maintain or improve test coverage
- Use table-driven tests where appropriate
- Mock external dependencies

### Documentation

- Update README.md if you add new features
- Add inline comments for complex logic
- Update API documentation if applicable

## AI-Assisted Development Guidelines

This template is designed to work well with AI coding assistants. When contributing:

### For AI-Friendly Code

- Write clear, descriptive function and variable names
- Include comprehensive comments explaining business logic
- Structure code in small, focused functions
- Use consistent naming conventions

### Documentation for AI Context

- Keep README.md up to date with clear examples
- Document configuration options thoroughly
- Include troubleshooting sections
- Provide clear setup instructions

## Release Process

This project uses semantic versioning and automated releases:

1. Changes are made via pull requests
2. Releases are triggered by pushing version tags
3. GoReleaser handles building and publishing
4. Release notes are generated automatically

## Getting Help

- Check existing issues and discussions
- Ask questions in GitHub Discussions
- Read the documentation thoroughly
- Look at existing code for examples

## Recognition

Contributors are recognized in:
- Release notes
- GitHub contributors list
- Special mentions for significant contributions

Thank you for helping make this template better for the Go and AI community!
