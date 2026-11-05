"""Run only against a disposable PostgreSQL database. Never use production credentials."""
import argparse
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import time


def run(*command, cwd=None, expected=True, **kwargs):
    result = subprocess.run(command, cwd=cwd, check=False, timeout=180, **kwargs)
    if (result.returncode == 0) != expected:
        raise RuntimeError(f"unexpected result from {Path(command[0]).name}: {result.returncode}")
    return result


def sql(statement):
    # PSQL may point to a local Docker wrapper. Only the isolated QA database is used.
    return run(os.environ.get("PSQL", "psql"), os.environ["DATABASE_URL"], "-X", "-q", "-v", "ON_ERROR_STOP=1",
               input=statement, text=True, stdout=subprocess.DEVNULL)


def scalar(statement):
    result = run(os.environ.get("PSQL", "psql"), os.environ["DATABASE_URL"], "-X", "-qAt", "-v", "ON_ERROR_STOP=1",
                 input=statement, text=True, stdout=subprocess.PIPE)
    return result.stdout.strip()


def interrupted_migration(args, root, version):
    migration = root / "migrations" / f"{version:06}_interrupted.up.sql"
    migration.write_text("SELECT pg_sleep(20);")
    process = subprocess.Popen([args.runner, "migrate"], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    try:
        deadline = time.monotonic() + 8
        active = "SELECT count(*) FROM pg_stat_activity WHERE application_name LIKE 'smart-todo-migrate-%' AND state='active' AND query LIKE 'SELECT pg_sleep%'"
        while scalar(active) != "1":
            if process.poll() is not None or time.monotonic() > deadline:
                raise RuntimeError("release command did not enter guarded migration")
            time.sleep(0.05)
        process.terminate()
        if process.wait(timeout=12) == 0 or scalar(active) != "0":
            raise RuntimeError("release command failed to stop its guarded database session")
        output = process.stdout.read()
        if "postgres://" in output or "ci-test-only" in output:
            raise RuntimeError("release cancellation leaked database credentials")
        if scalar("SELECT dirty FROM schema_migrations") != "t":
            raise RuntimeError("interrupted migration must retain dirty state")
    finally:
        if process.poll() is None:
            process.kill()
            process.wait(timeout=5)
        process.stdout.close()
        migration.unlink()


def wait_ready(process, path):
    deadline = time.monotonic() + 20
    while not path.exists():
        if process.poll() is not None or time.monotonic() >= deadline:
            raise RuntimeError("application compatibility fixture did not become ready")
        time.sleep(0.05)


def overlap(args, root, state, scenario):
    sql("DROP SCHEMA public CASCADE; CREATE SCHEMA public;")
    # The test copy only relocates the fixed migration path; production helper
    # source, cancellation guards and environment-only connection handling stay intact.
    (root / "migrations").rename(root / "candidate-migrations")
    shutil.copytree(Path(args.previous) / "internal/database/migrations", root / "migrations")
    try:
        run(str(root / "migrate"), "up")
    finally:
        shutil.rmtree(root / "migrations")
        (root / "candidate-migrations").rename(root / "migrations")
    # Bounded, reproducible data-volume smoke test, not production characterization.
    sql("""INSERT INTO users(id,email) VALUES ('20000000-0000-4000-8000-000000000001','volume@example.test');
        INSERT INTO todos(user_id,text)
        SELECT '20000000-0000-4000-8000-000000000001', 'volume-' || n FROM generate_series(1,1000) n;""")
    if scenario == "no-change":
        # Always exercise coexistence with no pending migrations, even when the
        # candidate release itself adds a migration. Both revisions use target N.
        run(args.runner, "migrate")
        no_change = json.loads((root / "release/contract.json").read_text())
        no_change["from_version"] = no_change["to_version"]
        (root / "release/contract.json").write_text(json.dumps(no_change))
    processes = []
    try:
        old = subprocess.Popen([args.old_binary, "old", str(state)])
        processes.append(old)
        wait_ready(old, state / "old.ready")
        plan = json.loads((root / "release/contract.json").read_text())
        if scenario == "expand":
            # Exercise a real expand migration even when this release has no schema change.
            version = plan["to_version"] + 1
            (root / "migrations" / f"{version:06}_compat_fixture.up.sql").write_text(
                "ALTER TABLE todos ADD COLUMN ci_overlap_note TEXT;\n")
            (root / "migrations" / f"{version:06}_compat_fixture.down.sql").write_text(
                "ALTER TABLE todos DROP COLUMN ci_overlap_note;\n")
            plan["to_version"] = version
            (root / "release/contract.json").write_text(json.dumps(plan))
        run(args.runner, "migrate")
        run(args.runner, "validate")
        if old.poll() is not None:
            raise RuntimeError("N-1 application failed during migration")
        new = subprocess.Popen([args.new_binary, "new", str(state)])
        processes.append(new)
        wait_ready(new, state / "new.ready")
        time.sleep(2)
        (state / "stop").touch()
        for process in processes:
            if process.wait(timeout=20) != 0:
                raise RuntimeError("application compatibility fixture failed")
        for label in ("old", "new"):
            counts = json.loads((state / f"{label}.json").read_text())
            if counts["api_writes"] < 2 or counts["peer_jobs_processed"] < 1:
                raise RuntimeError("insufficient concurrent API/worker coverage")
        run(args.runner, "migrate")  # Target already applied: no-op.
        run(args.runner, "validate")
    finally:
        for process in processes:
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=5)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for flag in ("previous", "migrate-source", "runner", "old-binary", "new-binary", "contract"):
        parser.add_argument("--" + flag, required=True)
    args = parser.parse_args()
    with tempfile.TemporaryDirectory(prefix="smart-todo-db-qa-") as directory:
        root = Path(directory)
        shutil.copytree("internal/database/migrations", root / "migrations")
        shutil.copytree("release", root / "release")
        shutil.copyfile(args.contract, root / "release/contract.json")
        helper_source = root / "helper-source"
        shutil.copytree(args.migrate_source, helper_source)
        helper_main = helper_source / "main.go"
        source = helper_main.read_text()
        if source.count('"file:///app/migrations"') != 1:
            raise RuntimeError("guarded helper migration path changed; review QA relocation")
        helper_main.write_text(source.replace('"file:///app/migrations"', json.dumps((root / "migrations").as_uri())))
        run("go", "build", "-mod=readonly", "-o", str(root / "migrate"), ".", cwd=helper_source)
        os.environ["RELEASE_ROOT"] = str(root)
        original_url = os.environ["DATABASE_URL"]
        try:
            # Supported helper options must not become PostgreSQL startup GUCs.
            os.environ["DATABASE_URL"] = original_url + "&x-statement-timeout=500"
            run(args.runner, "migrate")  # Empty bootstrap through guarded helper.
            run(args.runner, "validate")
            run(args.runner, "migrate")  # Same URL must also support no-change.
        finally:
            os.environ["DATABASE_URL"] = original_url
        plan = json.loads(Path(args.contract).read_text())
        sql("UPDATE schema_migrations SET dirty=true;")
        run(args.runner, "migrate", expected=False)
        run(args.runner, "validate", expected=False)
        sql("UPDATE schema_migrations SET dirty=false, version=999999;")
        run(args.runner, "migrate", expected=False)
        run(args.runner, "validate", expected=False)
        sql(f"UPDATE schema_migrations SET version={plan['to_version']};")
        sql("ALTER TABLE schema_migrations RENAME TO missing_version;")
        run(args.runner, "migrate", expected=False)
        sql("ALTER TABLE missing_version RENAME TO schema_migrations;")
        sql("INSERT INTO schema_migrations(version,dirty) VALUES (999999,false);")
        run(args.runner, "migrate", expected=False)
        run(args.runner, "validate", expected=False)
        sql("DELETE FROM schema_migrations WHERE version=999999;")
        invariant = root / "release/schema-invariants.sql"
        original = invariant.read_text()
        for query in ("SELECT false;", "SELECT true WHERE false;", "SELECT true UNION ALL SELECT true;",
                      "DELETE FROM todos RETURNING true;"):
            invariant.write_text(query)
            run(args.runner, "validate", expected=False)
        invariant.write_text(original)
        config_path = root / "release/validation.json"
        config = json.loads(config_path.read_text())
        config_path.write_text(json.dumps({**config, "statement_timeout_ms": 250}))
        invariant.write_text("SELECT pg_sleep(10) IS NULL;")
        started = time.monotonic()
        run(args.runner, "validate", expected=False)
        if time.monotonic() - started > 5:
            raise RuntimeError("statement timeout was not bounded")
        invariant.write_text(original)
        config_path.write_text(json.dumps(config))
        config_path.write_text(json.dumps({**config, "overall_timeout_seconds": 1}))
        invariant.write_text("SELECT pg_sleep(10) IS NULL;")
        started = time.monotonic()
        run(args.runner, "validate", expected=False)
        if time.monotonic() - started > 4:
            raise RuntimeError("overall process deadline was not bounded")
        invariant.write_text(original)
        config_path.write_text(json.dumps(config))
        # Prove lock limits independently of the shorter statement-timeout test.
        blocker = subprocess.Popen([os.environ.get("PSQL", "psql"), os.environ["DATABASE_URL"],
                                    "-X", "-qAt", "-v", "ON_ERROR_STOP=1"],
                                   stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
        try:
            blocker.stdin.write("BEGIN; LOCK TABLE todos IN ACCESS EXCLUSIVE MODE; SELECT 'locked'; SELECT pg_sleep(5); ROLLBACK;\n")
            blocker.stdin.close()
            if blocker.stdout.readline().strip() != "locked":
                raise RuntimeError("could not establish QA lock fixture")
            invariant.write_text("SELECT count(*) >= 0 FROM todos;")
            started = time.monotonic()
            run(args.runner, "validate", expected=False)
            if time.monotonic() - started > 4:
                raise RuntimeError("lock timeout was not bounded")
        finally:
            blocker.wait(timeout=10)
            blocker.stdout.close()
            invariant.write_text(original)
        for scenario in ("no-change", "release", "expand"):
            shutil.copyfile(args.contract, root / "release/contract.json")
            state = root / (scenario + "-state")
            state.mkdir()
            overlap(args, root, state, scenario)
        final_plan = json.loads((root / "release/contract.json").read_text())
        broken_version = final_plan["to_version"] + 1
        (root / "migrations" / f"{broken_version:06}_broken.up.sql").write_text("SELECT ci_missing_function();")
        final_plan["from_version"] = final_plan["to_version"]
        final_plan["to_version"] = broken_version
        (root / "release/contract.json").write_text(json.dumps(final_plan))
        (root / "migrations" / f"{broken_version:06}_broken.up.sql").unlink()
        interrupted_migration(args, root, broken_version)
        # Reset only the disposable fixture after proving interruption is dirty.
        sql(f"UPDATE schema_migrations SET version={final_plan['from_version']},dirty=false;")
        (root / "migrations" / f"{broken_version:06}_broken.up.sql").write_text("SELECT ci_missing_function();")
        run(args.runner, "migrate", expected=False)
        run(args.runner, "validate", expected=False)
        print("PASS: PostgreSQL bootstrap, replay, failure limits, N-1 migration and bidirectional API/worker overlap")


if __name__ == "__main__":
    main()
