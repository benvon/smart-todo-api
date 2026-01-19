#!/usr/bin/env bash

set -euo pipefail

: "${DATABASE_URL:?DATABASE_URL is not set}"

# Change to /app directory (explicit, even though WORKDIR should handle it)
cd /app || exit 1

echo "Starting database migration process..."

# Wait for database to be ready with retries
MAX_RETRIES=30
RETRY_INTERVAL=2
RETRY_COUNT=0

echo "Checking database connectivity..."

# Check database connectivity by parsing the error output
# If it's a connection error, we retry. If it's "no migrations" or "table doesn't exist", DB is ready.
check_db_connectivity() {
    local output
    output=$(/app/migrate -path /app/migrations -database "$DATABASE_URL" version 2>&1)
    local exit_code=$?
    
    # Exit code 0 means success (migrations exist and version check worked)
    if [ $exit_code -eq 0 ]; then
        return 0
    fi
    
    # Check error message - connection errors indicate DB not ready
    if echo "$output" | grep -qiE "(connection|connect|refused|timeout|no such host|network)"; then
        return 1  # Connection error - retry
    fi
    
    # Other errors (like "no change" or missing migrations table) mean DB is ready
    # This is expected on a fresh database
    return 0  # DB is ready
}

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if check_db_connectivity; then
        echo "Database is ready!"
        break
    fi
    
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -lt $MAX_RETRIES ]; then
        echo "Database not ready yet (attempt $RETRY_COUNT/$MAX_RETRIES). Retrying in ${RETRY_INTERVAL}s..."
        sleep $RETRY_INTERVAL
    else
        echo "ERROR: Database is not available after $MAX_RETRIES attempts"
        exit 1
    fi
done

# Check current migration version (for logging only)
echo "Checking current migration version..."
CURRENT_VERSION=$(/app/migrate -path /app/migrations -database "$DATABASE_URL" version 2>&1 || echo "unknown")
echo "Current database version: $CURRENT_VERSION"

# Run migrations - migrate up is idempotent and will only run pending migrations
# It returns exit code 0 both when migrations are applied and when there are no changes
echo "Running migrations..."
MIGRATE_OUTPUT=$(/app/migrate -path /app/migrations -database "$DATABASE_URL" up 2>&1)
MIGRATE_EXIT_CODE=$?

if [ $MIGRATE_EXIT_CODE -eq 0 ]; then
    # Check if migrations were applied or if we're already up to date
    if echo "$MIGRATE_OUTPUT" | grep -qiE "(no change|already at version)"; then
        echo "Database is already at the latest version. No migrations needed."
    else
        echo "$MIGRATE_OUTPUT"
        NEW_VERSION=$(/app/migrate -path /app/migrations -database "$DATABASE_URL" version 2>&1 || echo "unknown")
        echo "Migrations completed successfully! Database version: $NEW_VERSION"
    fi
    exit 0
else
    echo "ERROR: Migration failed with exit code $MIGRATE_EXIT_CODE"
    echo "$MIGRATE_OUTPUT"
    exit $MIGRATE_EXIT_CODE
fi
