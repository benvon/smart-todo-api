#!/usr/bin/env bash
set -euo pipefail

: "${DATABASE_URL:?DATABASE_URL is not set}"
cd /app
state_file=$(mktemp)
child=''
cleanup() { rm -f "$state_file"; }
terminate() {
    if [[ -n $child ]]; then
        kill -TERM "$child" 2>/dev/null || true
        wait "$child" 2>/dev/null || true
    fi
    exit 143
}
trap cleanup EXIT
trap terminate TERM INT

# Connection details stay in the environment. The packaged helper emits only
# version/clean-state tokens and error classes, never driver errors or SQL.
run_migrate() {
    local seconds=$1
    shift
    local kill_after=1 status=0
    [[ $1 != up ]] || kill_after=5
    timeout --signal=TERM --kill-after="${kill_after}s" "${seconds}s" /app/migrate "$@" 2>&1 &
    child=$!
    wait "$child" || status=$?
    child=''
    return "$status"
}

for ((attempt=1; attempt<=30; attempt++)); do
    if run_migrate 3 version >"$state_file"; then
        echo "Database state: $(cat "$state_file")"
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
    if run_migrate 5 version >"$state_file"; then
        echo "Migration complete: $(cat "$state_file")"
    else
        echo 'ERROR: Migration state could not be confirmed clean.'
        exit 1
    fi
else
    echo 'ERROR: Migration failed or exceeded its deadline; inspect schema before retrying.'
    exit 1
fi
