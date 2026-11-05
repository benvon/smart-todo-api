# Go quality tools

Go tool directives pin the linter, security scanners, and import formatter in this
module. Keeping its dependency graph separate avoids changing application or
migration-helper dependencies when a quality tool is upgraded. Mise manages the
Go compiler; it does not install these Go tools.

Run `make setup` and `make lint` from the repository root. Make, pre-commit, and CI
use `go tool -modfile=tools/quality/go.mod`, which builds the pinned tool with the
selected Go compiler and preserves the working directory of the code being checked.
No global tool binary or PATH fallback is used.

To update a tool, use (from the repository root):

```sh
go -C tools/quality get -tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
go -C tools/quality mod tidy
make setup lint security vulnerability-check
```

Commit both module files after validating the update. The npm security audit and
existing Go security scan options remain unchanged.
