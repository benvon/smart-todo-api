"""Regression matrix for the migration helper's configuration contract."""
import json
import os
import subprocess
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit


def quoted(value):
    return '"' + value.replace('"', '""') + '"'


def configured_url(base, options):
    parts = urlsplit(base)
    query = dict(parse_qsl(parts.query))
    query.update(options)
    return urlunsplit(parts._replace(query=urlencode(query)))


def readonly_checks(args, root, relation, schema, run, sql):
    sql(f"GRANT USAGE ON SCHEMA public TO ci_inspect; GRANT USAGE ON SCHEMA {quoted(schema)} TO ci_inspect; GRANT SELECT ON {relation} TO ci_inspect;")
    original = os.environ["DATABASE_URL"]
    parts = urlsplit(original)
    os.environ["DATABASE_URL"] = urlunsplit(parts._replace(netloc="ci_inspect:ci-inspect-test-only@" + parts.netloc.rsplit("@", 1)[-1]))
    try:
        run(args.runner, "validate")
        run(args.runner, "migrate")  # No-change must not require CREATE privilege.
    finally:
        os.environ["DATABASE_URL"] = original


def inspection_deadline(args, root, relation, run, scalar):
    blocker = subprocess.Popen([os.environ.get("PSQL", "psql"), os.environ["QA_DATABASE_URL"],
                                "-X", "-qAt", "-v", "ON_ERROR_STOP=1"],
                               stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True)
    try:
        blocker.stdin.write(f"BEGIN; LOCK TABLE {relation} IN ACCESS EXCLUSIVE MODE; SELECT 'locked'; SELECT pg_sleep(8); ROLLBACK;\n")
        blocker.stdin.close()
        if blocker.stdout.readline().strip() != "locked":
            raise RuntimeError("could not establish inspection deadline fixture")
        run(args.runner, "validate", expected=False)
        if scalar("SELECT count(*) FROM pg_stat_activity WHERE application_name LIKE 'smart-todo-migrate-%'") != "0":
            raise RuntimeError("timed-out inspection left a helper session")
        if blocker.poll() is not None:
            raise RuntimeError("inspection interrupted the unrelated blocking query")
    finally:
        blocker.wait(timeout=12)
        blocker.stdout.close()
    print("PASS: inspection deadline confirms session cleanup and preserves unrelated query", flush=True)


def exercise_configurations(args, root, run, sql, scalar):
    original_url = os.environ["DATABASE_URL"]
    original_contract = (root / "release/contract.json").read_text()
    plan = json.loads(original_contract)
    upgrade = plan["to_version"] + 1
    fixture = root / "migrations" / f"{upgrade:06}_configured_table_fixture.up.sql"
    # Setup/assertion connections are independent of the options under test.
    os.environ["QA_DATABASE_URL"] = original_url
    sql("DROP ROLE IF EXISTS ci_inspect; CREATE ROLE ci_inspect LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE PASSWORD 'ci-inspect-test-only';")
    cases = [
        ("custom", "public", "release_history", {"x-migrations-table": "release_history"}),
        ("default", "public", "schema_migrations", {}),
        ("literal-dot", "public", "release.history", {"x-migrations-table": "release.history"}),
        ("quoted", "public", "Release History", {"x-migrations-table": '"Release History"', "x-migrations-table-quoted": "true"}),
        ("qualified", "migration_meta", "Release.History", {"x-migrations-table": '"migration_meta"."Release.History"', "x-migrations-table-quoted": "true"}),
        ("qualified-spaces", "migration_meta", "Release.History", {"x-migrations-table": '"migration_meta" . "Release.History"', "x-migrations-table-quoted": "true"}),
        ("escaped", "migration_meta", 'Release"History', {"x-migrations-table": '"migration_meta"."Release""History"', "x-migrations-table-quoted": "true"}),
        ("long", "public", "migration_" + "a" * 64, {"x-migrations-table": "migration_" + "a" * 64}),
        ("search-path", "migration_meta", "release_history", {"x-migrations-table": "release_history", "options": "-c search_path=migration_meta,public"}),
    ]
    try:
        for name, schema, table, options in cases:
            os.environ["DATABASE_URL"] = original_url
            sql("DROP SCHEMA public CASCADE; CREATE SCHEMA public; DROP SCHEMA IF EXISTS migration_meta CASCADE; CREATE SCHEMA migration_meta;")
            (root / "release/contract.json").write_text(original_contract)
            if name == "search-path":
                # Existing app tables stay public; tracking follows search_path.
                run(args.runner, "migrate")
                sql("ALTER TABLE public.schema_migrations SET SCHEMA migration_meta; ALTER TABLE migration_meta.schema_migrations RENAME TO release_history;")
            os.environ["DATABASE_URL"] = configured_url(original_url, options)
            if name != "search-path":
                run(args.runner, "validate", expected=False)
                if scalar("SELECT count(*) FROM pg_tables WHERE schemaname IN ('public','migration_meta')") != "0":
                    raise RuntimeError("inspection must not create migration tracking tables")
            run(args.runner, "migrate")
            run(args.runner, "validate")
            relation = quoted(schema) + "." + quoted(table)
            if scalar(f"SELECT version FROM {relation} WHERE NOT dirty") != str(plan["to_version"]):
                raise RuntimeError("migration helper and release command disagree on tracking table")
            if name == "custom":
                inspection_deadline(args, root, relation, run, scalar)
            fixture.write_text("ALTER TABLE public.todos ADD COLUMN config_fixture integer;")
            changed = {**plan, "from_version": plan["to_version"], "to_version": upgrade}
            (root / "release/contract.json").write_text(json.dumps(changed))
            run(args.runner, "migrate")
            run(args.runner, "validate")
            readonly_checks(args, root, relation, schema, run, sql)
            if name != "default":
                sql("CREATE TABLE public.schema_migrations(version bigint, dirty boolean); INSERT INTO public.schema_migrations VALUES (999999,true);")
            run(args.runner, "migrate")  # A misleading default table cannot win.
            run(args.runner, "validate")
            for corruption in (
                f"UPDATE {relation} SET dirty=true;",
                f"UPDATE {relation} SET version=999999;",
                f"DELETE FROM {relation};",
                f"INSERT INTO {relation}(version,dirty) VALUES (999999,false);",
            ):
                sql(corruption)
                run(args.runner, "migrate", expected=False)
                run(args.runner, "validate", expected=False)
                sql(f"DELETE FROM {relation}; INSERT INTO {relation}(version,dirty) VALUES ({upgrade},false);")
            sql(f"DROP TABLE {relation};")
            run(args.runner, "migrate", expected=False)
            run(args.runner, "validate", expected=False)
            fixture.unlink()
            print(f"PASS: configured migration state: {name}", flush=True)
        os.environ["DATABASE_URL"] = original_url
        sql("DROP SCHEMA public CASCADE; CREATE SCHEMA public; DROP SCHEMA migration_meta CASCADE; CREATE SCHEMA migration_meta; CREATE TABLE migration_meta.keep_data(id integer); INSERT INTO migration_meta.keep_data VALUES (42);")
        run(args.runner, "migrate", expected=False)
        if scalar("SELECT count(*) FROM pg_tables WHERE schemaname='public'") != "0" or scalar("SELECT id FROM migration_meta.keep_data") != "42":
            raise RuntimeError("missing tracking on nonempty database must not bootstrap or modify data")
        print("PASS: missing tracking in a non-public populated schema is rejected without writes", flush=True)
    finally:
        os.environ["DATABASE_URL"] = original_url
        os.environ.pop("QA_DATABASE_URL", None)
        (root / "release/contract.json").write_text(original_contract)
        fixture.unlink(missing_ok=True)
        sql("DROP SCHEMA public CASCADE; CREATE SCHEMA public; DROP SCHEMA IF EXISTS migration_meta CASCADE;")
        sql("DROP OWNED BY ci_inspect; DROP ROLE ci_inspect;")
