# AI Agent Instructions

## General Guidelines

* Focus on "why" and not "how" in documentation.
* Test all return values for errors.
* Use `const` for variables that are not reassigned.
* Use short, imperative commit messages.
* Separate unrelated changes into distinct commits.
* Whenever possible, separate logic from dependencies or interfaces such that logic can be unit tested.

## Code Style

* Go version 1.23+ with the latest patch release.
* Avoid complex embedded logic in a single function. Break complex functions down into smaller functions.
* Use strong typing wherever possible.
* Don't use the `interface{}` construct. Use `any` instead.
* Follow Go naming conventions (e.g., `ID` not `Id`, `URL` not `Url`)
* Use meaningful variable names that describe intent
* Keep functions small and focused (ideally under 20 lines)

## Testing

* Write tests for all exported functions
* Use table-driven tests for multiple test cases
* Mock external dependencies using interfaces
* Aim for >80% test coverage
* Test both success and error paths
* Use `t.Parallel()` for independent tests

## Error Handling

* Always check error return values
* Use `errors.Is()` and `errors.As()` for error comparison
* Wrap errors with context using `fmt.Errorf("doing X: %w", err)`
* Don't ignore errors with `_`

## Performance

* Use `sync.Pool` for frequently allocated objects
* Prefer `strings.Builder` over string concatenation
* Use `strconv` over `fmt` for basic conversions
* Profile before optimizing

## Project Structure

* Use standard Go project layout:
  - `cmd/` for main applications
  - `internal/` for private application code
  - `pkg/` for public libraries
  - `api/` for API definitions
  - `docs/` for documentation
* Keep packages focused and cohesive
* Avoid circular dependencies

## Documentation

* Write clear, descriptive function and variable names
* Include comprehensive comments explaining business logic
* Document exported functions with proper Go doc comments
* Keep README.md up to date with clear examples
* Document configuration options thoroughly
* Include troubleshooting sections
* Provide clear setup instructions

## Dependencies

* Minimize external dependencies
* Use Go modules for dependency management
* Pin dependency versions in go.mod
* Regularly update dependencies for security patches
* Use `go mod tidy` to clean up unused dependencies

## Security

* Validate all input data
* Use HTTPS in production
* Implement proper authentication and authorization
* Follow OWASP guidelines
* Use security scanning tools (gosec, govulncheck)
* Never log sensitive information

## Logging and Monitoring

* Use structured logging (e.g., logrus, zap)
* Include correlation IDs for request tracing
* Log at appropriate levels (debug, info, warn, error)
* Implement health checks and metrics
* Use context for request cancellation and timeouts 