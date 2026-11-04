#!/usr/bin/env bash
set -euo pipefail
# Catch silent set -e exits, misclassified failures, unbounded retries, and DSN leaks.
root=$(cd "$(dirname "$0")/../.." && pwd)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/app" "$tmp/bin"
# Relocate fixed image paths in the test copy only; production has no path overrides.
sed "s|/app|$tmp/app|g" "$root/scripts/run_migrations.sh" > "$tmp/run.sh"
cat > "$tmp/bin/sleep" <<'SH'
#!/usr/bin/env bash
exit 0
SH
cat > "$tmp/bin/timeout" <<'SH'
#!/usr/bin/env bash
shift 3
exec "$@"
SH
cat > "$tmp/app/migrate" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "$TEST_CALLS"
command=${!#}
case "$TEST_CASE:$command" in
  connection_retry_exhausted:*) echo "error: dial tcp: connection refused $DATABASE_URL"; exit 2 ;;
  authentication_denied:*) echo "error: pq: password authentication failed for user fake_user $DATABASE_URL"; exit 1 ;;
  invalid_url:*) echo "error: parse $DATABASE_URL: invalid URL escape"; exit 1 ;;
  dirty_schema:version) echo 'error=schema'; exit 1 ;;
  fresh_database:version) if [[ ! -f "$TEST_STATE" ]]; then echo 'version=none clean=true'; exit 0; fi ;;
  postcheck_failure:version) if [[ -f "$TEST_STATE" ]]; then echo "error: $DATABASE_URL unavailable"; exit 1; fi ;;
  migration_failure:up) echo "error: migration failed: invalid SQL $DATABASE_URL"; exit 1 ;;
  command_timeout:up) exit 124 ;;
  credential_redaction:up) echo "untrusted success $DATABASE_URL fake_password fake_user"; touch "$TEST_STATE"; exit 0 ;;
  already_current:up) echo 'no change'; touch "$TEST_STATE"; exit 0 ;;
  *:up) touch "$TEST_STATE"; echo '1/u initial'; exit 0 ;;
esac
echo 'version=1 clean=true'
SH
chmod +x "$tmp/app/migrate" "$tmp/bin/sleep" "$tmp/bin/timeout"
failures=0
for scenario in success already_current fresh_database connection_retry_exhausted authentication_denied dirty_schema migration_failure credential_redaction invalid_url command_timeout postcheck_failure; do
  : > "$tmp/calls"
  rm -f "$tmp/state"
  status=0
  TEST_CASE=$scenario TEST_CALLS="$tmp/calls" TEST_STATE="$tmp/state" PATH="$tmp/bin:$PATH" \
    DATABASE_URL='postgres://fake_user:fake_password@invalid.example/fake' \
    bash "$tmp/run.sh" > "$tmp/output" 2>&1 || status=$?
  bad=0
  case "$scenario" in
    success|already_current|fresh_database|credential_redaction)
      [[ $status -eq 0 ]] || bad=1
      grep -q 'clean=true' "$tmp/output" || bad=1 ;;
    *) [[ $status -ne 0 ]] || bad=1
       grep -q 'ERROR:' "$tmp/output" || bad=1 ;;
  esac
  if grep -qE 'fake_password|fake_user|postgres://' "$tmp/output" "$tmp/calls"; then bad=1; fi
  calls=$(wc -l < "$tmp/calls")
  [[ $calls -le 32 ]] || bad=1
  case "$scenario" in
    connection_retry_exhausted) [[ $calls -eq 30 ]] || bad=1 ;;
    authentication_denied|dirty_schema|invalid_url) [[ $calls -eq 1 ]] || bad=1 ;;
  esac
  if [[ $bad -eq 1 ]]; then echo "FAIL: $scenario (exit=$status calls=$calls)"; failures=$((failures+1)); else echo "PASS: $scenario"; fi
done
[[ $failures -eq 0 ]]
