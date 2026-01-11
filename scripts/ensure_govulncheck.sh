#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -lt 1 ]; then
	echo "Usage: $0 <version> [govulncheck args...]" >&2
	exit 2
fi

VERSION="$1"
shift
MODULE_VERSION="v${VERSION}"
ARGS=("$@")
if [ "${#ARGS[@]}" -eq 0 ]; then
	ARGS=(-version)
fi

if command -v govulncheck >/dev/null 2>&1 && govulncheck -version >/dev/null 2>&1; then
	exec govulncheck "${ARGS[@]}"
fi

if command -v asdf >/dev/null 2>&1; then
	asdf plugin add govulncheck >/dev/null 2>&1 || true
	if asdf install govulncheck "${VERSION}" >/dev/null 2>&1; then
		asdf reshim >/dev/null 2>&1 || true
		if command -v govulncheck >/dev/null 2>&1 && govulncheck -version >/dev/null 2>&1; then
			exec govulncheck "${ARGS[@]}"
		fi
	fi
fi

if [ "${GOVULNCHECK_QUIET:-0}" != "1" ]; then
	echo "govulncheck binary not available; running via go run ${MODULE_VERSION}." >&2
fi
exec env GO111MODULE=on go run "golang.org/x/vuln/cmd/govulncheck@${MODULE_VERSION}" -- "${ARGS[@]}"
