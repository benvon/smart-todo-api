#!/usr/bin/env bash
set -euo pipefail

: "${DATABASE_URL:?DATABASE_URL is not set}"
cd /app

# Connection details stay in the environment. The packaged helper emits only
# version/clean-state tokens and error classes, never driver errors or SQL.
run_migrate() {
    local seconds=$1
    shift
    timeout --signal=TERM --kill-after=1s "${seconds}s" /app/migrate "$@" 2>&1
}

for ((attempt=1; attempt<=30; attempt++)); do
    if state=$(run_migrate 3 version); then
        echo "Database state: $state"
        break
    else
        status=$?
    fi
    # Only a typed connectivity failure or a probe deadline warrants retry.
    if [[ $status -ne 2 && $status -ne 124 ]]; then
        echo 'ERROR: Database preflight failed (configuration, authorization, or schema).'
        exit 1
    fi
    if [[ $attempt -eq 30 ]]; then
        echo 'ERROR: Database unavailable after 30 bounded attempts.'
        exit 1
    fi
    echo "Database unavailable (attempt $attempt/30). Retrying..."
    sleep 2
done

# The worst-case retry/kill budget plus up and postcheck fits the 300-second
# Job deadline. A partially applied migration is never retried here.
if run_migrate 100 up >/dev/null; then
    if state=$(run_migrate 5 version); then
        echo "Migration complete: $state"
    else
        echo 'ERROR: Migration state could not be confirmed clean.'
        exit 1
    fi
else
    echo 'ERROR: Migration failed or exceeded its deadline; inspect schema before retrying.'
    exit 1
fi
