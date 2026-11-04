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
    --env-file "$tmp/postgres.env" postgres:16 postgres -c log_connections=on >/dev/null
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
mkdir "$tmp/timeout"
printf '%s\n' 'SELECT pg_sleep(200);' > "$tmp/timeout/1_timeout.up.sql"
chmod 755 "$tmp" "$tmp/timeout"
chmod 644 "$tmp/timeout/1_timeout.up.sql"
docker exec "$name" psql -U fake_admin -d postgres -c 'CREATE DATABASE fake_signal OWNER fake_user' >/dev/null
printf '%s\n' 'DATABASE_URL=postgres://fake_user:fake_password@database:5432/fake_signal?sslmode=disable' > "$tmp/app.env"
docker run -d --name "$name-probe" --network "$network" --env-file "$tmp/app.env" \
    --mount "type=bind,src=$tmp/timeout,dst=/app/migrations,readonly" \
    --entrypoint /app/run_migrations.sh "$image" >/dev/null
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
stop_deadline=$((SECONDS+10))
while [[ $(docker inspect --format '{{.State.Running}}' "$name-probe") == true && $SECONDS -lt $stop_deadline ]]; do sleep 0.1; done
[[ $(docker inspect --format '{{.State.Running}}' "$name-probe") == false ]] || { echo 'FAIL: wrapper did not terminate helper within 10s'; exit 1; }
[[ $(docker wait "$name-probe") != 0 ]] || { echo 'FAIL: interrupted helper reported success'; exit 1; }
active=$(docker exec "$name" psql -U fake_admin -d postgres -Atc "SELECT count(*) FROM pg_stat_activity WHERE datname='fake_signal' AND application_name LIKE 'smart-todo-migrate-%'")
[[ $active == 0 ]] || { echo 'FAIL: SIGTERM left migration query active'; exit 1; }
dirty=$(docker exec "$name" psql -U fake_user -d fake_signal -Atc 'SELECT dirty FROM schema_migrations')
[[ $dirty == t ]] || { echo 'FAIL: SIGTERM cleared dirty state'; exit 1; }
docker logs "$name-probe" > "$tmp/signal-output" 2>&1
if grep -qE 'fake_password|fake_user|postgres://' "$tmp/signal-output"; then echo 'FAIL: SIGTERM leaked credentials'; exit 1; fi
other=$(docker exec "$name" psql -U fake_admin -d postgres -Atc "SELECT count(*) FROM pg_stat_activity WHERE datname='fake' AND state='active' AND query LIKE '%unrelated synthetic query%'")
[[ $other == 1 ]] || { echo 'FAIL: unrelated query interrupted'; exit 1; }
echo 'PASS: ordinary-role SIGTERM cancels exact migration session, retains dirty state, and preserves unrelated query'
docker rm "$name-probe" >/dev/null
# A FIFO holds source loading after version 1 is clean and the backend is idle.
# TERM must prevent version 2 from starting when that source later becomes ready.
for gap_mode in idle buffered; do
    gap_database="fake_gap_$gap_mode"
    expected_gap=1:false
    [[ $gap_mode != buffered ]] || expected_gap=2:true
    docker exec "$name" psql -U fake_admin -d postgres -c "CREATE DATABASE $gap_database OWNER fake_user" >/dev/null
    printf '%s\n' "DATABASE_URL=postgres://fake_user:fake_password@database:5432/$gap_database?sslmode=disable" > "$tmp/app.env"
    docker run -d --name "$name-probe" --network "$network" --env-file "$tmp/app.env" --entrypoint bash "$image" -c '
        mv /app/migrations /app/original_migrations
        mkdir /app/migrations
        printf "%s\n" "CREATE TABLE gap_first(id integer);" > /app/migrations/1_first.up.sql
        mkfifo /app/migrations/2_gap.up.sql
        exec /app/run_migrations.sh
    ' >/dev/null
    if [[ $gap_mode == buffered ]]; then
        docker exec -d "$name-probe" bash -c '
            while [[ ! -p /app/migrations/2_gap.up.sql ]]; do sleep 0.1; done
            exec 3>/app/migrations/2_gap.up.sql
            while [[ ! -f /tmp/release_body ]]; do sleep 0.1; done
            printf "%s\n" "CREATE TABLE must_not_start(id integer);" >&3
            exec 3>&-
        '
    fi
    gap=''
    for ((i=0;i<30;i++)); do
        gap=$(docker exec "$name" psql -U fake_user -d "$gap_database" -Atc "SELECT version||':'||dirty FROM schema_migrations" 2>/dev/null || true)
        [[ $gap == "$expected_gap" ]] && break
        sleep 0.2
    done
    [[ $gap == "$expected_gap" ]] || { echo "FAIL: $gap_mode gap not reached (state=$gap expected=$expected_gap)"; exit 1; }
    active=$(docker exec "$name" psql -U fake_admin -d postgres -Atc "SELECT count(*) FROM pg_stat_activity WHERE datname='$gap_database' AND state='active'")
    [[ $active == 0 ]] || { echo 'FAIL: gap was not idle'; exit 1; }
    docker kill --signal TERM "$name-probe" >/dev/null
    sleep 0.5
    if [[ $gap_mode == buffered ]]; then
        docker exec "$name-probe" touch /tmp/release_body
    else
        docker exec "$name-probe" bash -c 'printf "%s\n" "CREATE TABLE must_not_start(id integer);" > /app/migrations/2_gap.up.sql'
    fi
    stop_deadline=$((SECONDS+10))
    while [[ $(docker inspect --format '{{.State.Running}}' "$name-probe") == true && $SECONDS -lt $stop_deadline ]]; do sleep 0.1; done
    [[ $(docker inspect --format '{{.State.Running}}' "$name-probe") == false ]] || { echo 'FAIL: gap cancellation did not finish'; exit 1; }
    [[ $(docker wait "$name-probe") != 0 ]] || { echo 'FAIL: gap cancellation reported success'; exit 1; }
    next=$(docker exec "$name" psql -U fake_user -d "$gap_database" -Atc "SELECT count(*) FROM information_schema.tables WHERE table_name='must_not_start'")
    gap=$(docker exec "$name" psql -U fake_user -d "$gap_database" -Atc "SELECT version||':'||dirty FROM schema_migrations")
    [[ $next == 0 && $gap == "$expected_gap" ]] || { echo 'FAIL: SQL or clean version advanced after gap cancellation'; exit 1; }
    active=$(docker exec "$name" psql -U fake_admin -d postgres -Atc "SELECT count(*) FROM pg_stat_activity WHERE datname='$gap_database' AND application_name LIKE 'smart-todo-migrate-%'")
    [[ $active == 0 ]] || { echo 'FAIL: canceled gap session remained'; exit 1; }
    docker rm "$name-probe" >/dev/null
    echo "PASS: entrypoint $gap_mode gap cancellation prevents SQL and successful version update"
done
printf '%s\n' 'DATABASE_URL=postgres://fake_user:fake_password@database:5432/fake?sslmode=disable' > "$tmp/app.env"
# Rejected parser/options must not open even a transient database session.
docker exec "$name" psql -U fake_admin -d postgres -c 'CREATE DATABASE fake_unsupported OWNER fake_user' >/dev/null
for option in x-multi-statement=true x-multi-statement=1 x-multi-statement=invalid x-unknown-option=value; do
    printf '%s\n' "DATABASE_URL=postgres://fake_user:fake_password@database:5432/fake_unsupported?sslmode=disable&$option" > "$tmp/unsupported.env"
    status=0
    docker run --rm --network "$network" --env-file "$tmp/unsupported.env" --entrypoint /app/migrate "$image" version > "$tmp/unsupported-output" 2>&1 || status=$?
    if [[ $status != 1 ]] || ! grep -q '^error=unsupported_option$' "$tmp/unsupported-output"; then echo 'FAIL: unsupported option outcome'; exit 1; fi
    if grep -qE 'fake_password|fake_user|postgres://' "$tmp/unsupported-output"; then echo 'FAIL: unsupported option leaked credentials'; exit 1; fi
done
docker logs "$name" > "$tmp/postgres-log" 2>&1
grep -qE 'connection authorized:.*database=fake_signal([[:space:]]|$)' "$tmp/postgres-log" || { echo 'FAIL: fixture connection logging not observed'; exit 1; }
if grep -qE 'connection authorized:.*database=fake_unsupported([[:space:]]|$)' "$tmp/postgres-log"; then echo 'FAIL: unsupported option opened a DB session'; exit 1; fi
echo 'PASS: unsupported migration options rejected before any database session'
run_case fresh_schema success
version=$(docker exec "$name" psql -U fake_user -d fake -Atc 'SELECT version FROM schema_migrations WHERE NOT dirty')
[[ $version =~ ^[0-9]+$ ]] || { echo 'FAIL: schema not clean'; exit 1; }
run_case already_current success
# Block the real version query beyond its internal 2s deadline, then let the
# wrapper retry until the synthetic lock expires. The helper must exit 2.
docker exec -d "$name" psql -U fake_admin -d fake -c 'BEGIN; LOCK TABLE schema_migrations IN ACCESS EXCLUSIVE MODE; SELECT pg_sleep(8) /* readiness deadline fixture */; COMMIT;'
locked=0
for ((i=0;i<20;i++)); do
    locked=$(docker exec "$name" psql -U fake_admin -d postgres -Atc "SELECT count(*) FROM pg_stat_activity WHERE datname='fake' AND state='active' AND query LIKE '%readiness deadline fixture%'")
    [[ $locked == 1 ]] && break
    sleep 0.1
done
[[ $locked == 1 ]] || { echo 'FAIL: readiness lock not acquired'; exit 1; }
status=0
docker run --rm --network "$network" --env-file "$tmp/app.env" --entrypoint /app/migrate "$image" version > "$tmp/probe-deadline" 2>&1 || status=$?
[[ $status == 2 ]] || { echo "FAIL: internal probe deadline status=$status"; exit 1; }
if grep -qE 'fake_password|fake_user|postgres://' "$tmp/probe-deadline"; then echo 'FAIL: deadline leaked credentials'; exit 1; fi
run_case internal_probe_deadline_recovery success
echo 'PASS: internal readiness deadline exits2 and wrapper retries to success'
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
printf '%s\n' 'DATABASE_URL=postgres://fake_user:fake_password@database:5432/fake_timeout?sslmode=disable' > "$tmp/app.env"
start=$SECONDS
run_case migration_timeout failure --mount "type=bind,src=$tmp/timeout,dst=/app/migrations,readonly"
[[ $((SECONDS-start)) -ge 85 && $((SECONDS-start)) -lt 115 ]] || { echo 'FAIL: migration exceeded timeout budget'; exit 1; }
dirty=$(docker exec "$name" psql -U fake_user -d fake_timeout -Atc 'SELECT dirty FROM schema_migrations')
[[ $dirty == t ]] || { echo 'FAIL: timeout did not retain dirty state'; exit 1; }
sleep 1
active=$(docker exec "$name" psql -U fake_user -d fake -Atc "SELECT count(*) FROM pg_stat_activity WHERE datname='fake_timeout' AND state='active'")
[[ $active == 0 ]] || { echo 'FAIL: timed-out migration left an active query'; exit 1; }
echo 'PASS: real PostgreSQL clean/dirty, no-change, deadline, and query termination contracts'
