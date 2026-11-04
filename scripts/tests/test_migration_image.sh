#!/usr/bin/env bash
set -euo pipefail
# Accept only an existing local image. All database targets are created here,
# isolated from host ports and removed by exact generated names.
[[ $# -eq 1 ]] || { echo 'Usage: test_migration_image.sh LOCAL_IMAGE'; exit 2; }
image=$(docker image inspect --format '{{.Id}}' "$1")
tmp=$(mktemp -d)
name="smart-todo-migration-test-$(basename "$tmp" | tr '[:upper:]' '[:lower:]')"
network="$name-net"
cleanup() {
    docker rm -f "$name" "$name-source" "$name-probe" >/dev/null 2>&1 || true
    docker network rm "$network" >/dev/null 2>&1 || true
    rm -rf "$tmp"
}
trap cleanup EXIT
# No caller-supplied host, port, URL, credentials, Docker args or mount paths.
docker network create --internal "$network" >/dev/null
printf '%s\n' 'POSTGRES_USER=fake_admin' 'POSTGRES_PASSWORD=fake_password' 'POSTGRES_DB=postgres' > "$tmp/postgres.env"
docker run -d --name "$name" --network "$network" --network-alias database \
    --env-file "$tmp/postgres.env" postgres:16 >/dev/null
ready=false
for ((i=0; i<60; i++)); do
    if docker exec "$name" pg_isready -U fake_admin -d postgres >/dev/null 2>&1; then ready=true; break; fi
    sleep 1
done
[[ $ready == true ]] || { echo 'FAIL: disposable PostgreSQL unavailable'; exit 1; }
docker exec -i "$name" psql -U fake_admin -d postgres >/dev/null <<'SQL'
CREATE ROLE fake_user LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD 'fake_password';
CREATE DATABASE fake OWNER fake_user;
SQL
printf '%s\n' 'DATABASE_URL=postgres://fake_user:fake_password@database:5432/fake?sslmode=disable' > "$tmp/app.env"
run_case() {
    local scenario=$1 expected=$2 status=0
    shift 2
    docker run --rm --network "$network" --env-file "$tmp/app.env" \
        "$@" --entrypoint /app/run_migrations.sh "$image" > "$tmp/output" 2>&1 || status=$?
    if grep -qE 'fake_password|fake_user|postgres://' "$tmp/output"; then echo "FAIL: $scenario leaked credentials"; exit 1; fi
    if [[ $expected == success ]]; then
        if [[ $status -ne 0 ]] || ! grep -q 'clean=true' "$tmp/output"; then
            echo "FAIL: $scenario exit=$status"
            cat "$tmp/output"
            exit 1
        fi
    else
        if [[ $status -eq 0 ]] || ! grep -q 'ERROR:' "$tmp/output"; then
            echo "FAIL: $scenario exit=$status"
            exit 1
        fi
    fi
    echo "PASS: $scenario"
}
run_case fresh_schema success
version=$(docker exec "$name" psql -U fake_user -d fake -Atc 'SELECT version FROM schema_migrations WHERE NOT dirty')
[[ $version =~ ^[0-9]+$ ]] || { echo 'FAIL: schema not clean'; exit 1; }
run_case already_current success
unchanged=$(docker exec "$name" psql -U fake_user -d fake -Atc 'SELECT version FROM schema_migrations WHERE NOT dirty')
[[ $unchanged == "$version" ]] || { echo 'FAIL: no-change modified schema version'; exit 1; }
# Copy migrations from the tested image, never assume this checkout matches it.
mkdir "$tmp/invalid"
docker create --name "$name-source" "$image" >/dev/null
docker cp "$name-source:/app/migrations/." "$tmp/invalid/"
docker rm "$name-source" >/dev/null
chmod 755 "$tmp" "$tmp/invalid"
printf '%s\n' 'SELECT pg_sleep(8);' > "$tmp/invalid/$((version+1))_argv.up.sql"
chmod 644 "$tmp/invalid/"*.sql
docker run -d --name "$name-probe" --network "$network" --env-file "$tmp/app.env" \
    --mount "type=bind,src=$tmp/invalid,dst=/app/migrations,readonly" \
    --entrypoint /app/run_migrations.sh "$image" >/dev/null
sleep 2
docker exec "$name-probe" bash -c 'for file in /proc/[0-9]*/cmdline; do cat "$file"; echo; done' > "$tmp/argv"
grep -aq '^/app/migrate' "$tmp/argv" || { echo 'FAIL: running helper not observed'; exit 1; }
if grep -aqE 'fake_password|fake_user|postgres://' "$tmp/argv"; then echo 'FAIL: DSN in process argv'; exit 1; fi
[[ $(docker wait "$name-probe") == 0 ]] || { echo 'FAIL: argv probe migration failed'; exit 1; }
docker logs "$name-probe" > "$tmp/probe-output" 2>&1
if grep -qE 'fake_password|fake_user|postgres://' "$tmp/probe-output"; then echo 'FAIL: DSN in process logs'; exit 1; fi
docker rm "$name-probe" >/dev/null
echo 'PASS: DSN absent from running migration argv and logs'
version=$((version+1))
# The invalid SQL has a fake credential marker to catch raw SQL logging.
printf '%s\n' 'INVALID SQL fake_password;' > "$tmp/invalid/$((version+1))_invalid.up.sql"
chmod 644 "$tmp/invalid/"*.sql
run_case invalid_migration failure --mount "type=bind,src=$tmp/invalid,dst=/app/migrations,readonly"
dirty=$(docker exec "$name" psql -U fake_user -d fake -Atc 'SELECT dirty FROM schema_migrations')
[[ $dirty == t ]] || { echo 'FAIL: invalid migration did not retain dirty state'; exit 1; }
run_case dirty_schema failure
printf '%s\n' 'DATABASE_URL=postgres://fake_user:wrong_fake_password@database:5432/fake?sslmode=disable' > "$tmp/app.env"
run_case authentication_denied failure
printf '%s\n' 'DATABASE_URL=postgres://fake_user:fake_password@database:5432/fake?sslmode=verify-full' > "$tmp/app.env"
run_case tls_failure failure
printf '%s\n' 'DATABASE_URL=postgres://fake_user:fake_password@%invalid/fake' > "$tmp/app.env"
run_case invalid_url failure
printf '%s\n' 'DATABASE_URL=postgres://fake_user:fake_password@database:1/fake?sslmode=disable' > "$tmp/app.env"
start=$SECONDS
run_case unavailable_database failure
[[ $((SECONDS-start)) -lt 120 ]] || { echo 'FAIL: connection retries exceeded bound'; exit 1; }
# A separate fresh disposable database demonstrates the actual packaged timeout,
# dirty marker, and termination of the interrupted PostgreSQL statement.
docker exec "$name" psql -U fake_admin -d postgres -c 'CREATE DATABASE fake_timeout OWNER fake_user' >/dev/null
mkdir "$tmp/timeout"
printf '%s\n' 'SELECT pg_sleep(200);' > "$tmp/timeout/1_timeout.up.sql"
chmod 755 "$tmp/timeout"
chmod 644 "$tmp/timeout/1_timeout.up.sql"
printf '%s\n' 'DATABASE_URL=postgres://fake_user:fake_password@database:5432/fake_timeout?sslmode=disable' > "$tmp/app.env"
start=$SECONDS
run_case migration_timeout failure --mount "type=bind,src=$tmp/timeout,dst=/app/migrations,readonly"
[[ $((SECONDS-start)) -ge 85 && $((SECONDS-start)) -lt 115 ]] || { echo 'FAIL: migration exceeded timeout budget'; exit 1; }
dirty=$(docker exec "$name" psql -U fake_user -d fake_timeout -Atc 'SELECT dirty FROM schema_migrations')
[[ $dirty == t ]] || { echo 'FAIL: timeout did not retain dirty state'; exit 1; }
sleep 1
active=$(docker exec "$name" psql -U fake_user -d fake -Atc "SELECT count(*) FROM pg_stat_activity WHERE datname='fake_timeout' AND state='active'")
[[ $active == 0 ]] || { echo 'FAIL: timed-out migration left an active query'; exit 1; }
docker exec "$name" psql -U fake_admin -d postgres -c 'CREATE DATABASE fake_signal OWNER fake_user' >/dev/null
printf '%s\n' 'DATABASE_URL=postgres://fake_user:fake_password@database:5432/fake_signal?sslmode=disable' > "$tmp/app.env"
docker run -d --name "$name-probe" --network "$network" --env-file "$tmp/app.env" \
    --mount "type=bind,src=$tmp/timeout,dst=/app/migrations,readonly" \
    --entrypoint /app/migrate "$image" up >/dev/null
active=0
for ((i=0;i<30;i++)); do
    active=$(docker exec "$name" psql -U fake_admin -d postgres -Atc "SELECT count(*) FROM pg_stat_activity WHERE datname='fake_signal' AND state='active' AND query LIKE 'SELECT pg_sleep%'")
    [[ $active == 1 ]] && break
    sleep 1
done
[[ $active == 1 ]] || { echo 'FAIL: signal test did not enter migration'; exit 1; }
docker exec -d "$name" psql -U fake_user -d fake -c 'SELECT pg_sleep(30) /* unrelated synthetic query */'
other=0
for ((i=0;i<10;i++)); do
    other=$(docker exec "$name" psql -U fake_admin -d postgres -Atc "SELECT count(*) FROM pg_stat_activity WHERE datname='fake' AND state='active' AND query LIKE '%unrelated synthetic query%'")
    [[ $other == 1 ]] && break
    sleep 1
done
[[ $other == 1 ]] || { echo 'FAIL: unrelated query not running'; exit 1; }
docker kill --signal TERM "$name-probe" >/dev/null
[[ $(docker wait "$name-probe") != 0 ]] || { echo 'FAIL: interrupted helper reported success'; exit 1; }
active=$(docker exec "$name" psql -U fake_admin -d postgres -Atc "SELECT count(*) FROM pg_stat_activity WHERE datname='fake_signal' AND state='active'")
[[ $active == 0 ]] || { echo 'FAIL: SIGTERM left migration query active'; exit 1; }
dirty=$(docker exec "$name" psql -U fake_user -d fake_signal -Atc 'SELECT dirty FROM schema_migrations')
[[ $dirty == t ]] || { echo 'FAIL: SIGTERM cleared dirty state'; exit 1; }
docker logs "$name-probe" > "$tmp/signal-output" 2>&1
if grep -qE 'fake_password|fake_user|postgres://' "$tmp/signal-output"; then echo 'FAIL: SIGTERM leaked credentials'; exit 1; fi
other=$(docker exec "$name" psql -U fake_admin -d postgres -Atc "SELECT count(*) FROM pg_stat_activity WHERE datname='fake' AND state='active' AND query LIKE '%unrelated synthetic query%'")
[[ $other == 1 ]] || { echo 'FAIL: unrelated query interrupted'; exit 1; }
echo 'PASS: ordinary-role SIGTERM cancels exact migration session, retains dirty state, and preserves unrelated query'
echo 'PASS: real PostgreSQL clean/dirty, no-change, deadline, and query termination contracts'
