#!/usr/bin/env bash

: "${SERVER_PORT:?SERVER_PORT is not set}"
: "${BASE_URL:?BASE_URL is not set}"
: "${FRONTEND_URL:?FRONTEND_URL is not set}"
# : "${OPENAI_API_KEY:?OPENAI_API_KEY is not set}"

# Change to /app directory (explicit, even though WORKDIR should handle it)
cd /app || exit 1

# Start the server
exec /app/server