#!/usr/bin/env bash
set -euo pipefail

# The image carries its supported starting/target schemas and finite limits.
# Never force a dirty schema or roll back automatically.
exec /app/db-release migrate
