"""Build the versioned GitOps handoff from the release revision, not live state."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import subprocess
import tarfile

MIGRATIONS = "internal/database/migrations"
STABLE = re.compile(r"v(\d+)\.(\d+)\.(\d+)$")
MANDATORY = {"quality", "cross-build", "web-quality", "database-compatibility"}


def git(*args):
    return subprocess.check_output(["git", *args], text=True).strip()


def select_baseline(releases, ref):
    current = re.fullmatch(r"v(\d+)\.(\d+)\.(\d+)(?:-rc\d+)?", ref)
    ceiling = tuple(map(int, current.groups())) if current else None
    candidates = []
    for item in releases:
        match = STABLE.fullmatch(item["tag_name"])
        if match and not item["draft"] and not item["prerelease"]:
            version = tuple(map(int, match.groups()))
            if ceiling is None or version < ceiling:
                candidates.append((version, item["tag_name"]))
    if not candidates:
        raise ValueError("no previous published stable release; maintainer must establish a baseline")
    return max(candidates)[1]


def inventory(directory):
    items, seen = [], set()
    for path in sorted(directory.iterdir()):
        match = re.fullmatch(r"(\d+)_.+\.(up|down)\.sql", path.name)
        if not match or not path.is_file() or path.is_symlink():
            raise ValueError(f"invalid migration file: {path.name}")
        key = (int(match[1]), match[2])
        if key in seen or key[0] < 1:
            raise ValueError(f"duplicate or invalid migration version: {path.name}")
        seen.add(key)
        items.append({"file": path.name, "version": key[0], "direction": key[1],
                      "sha256": hashlib.sha256(path.read_bytes()).hexdigest()})
    versions = {version for version, _ in seen}
    if not versions or any((v, d) not in seen for v in versions for d in ("up", "down")):
        raise ValueError("every migration requires one up/down pair")
    return items


def check_history(old, current):
    for name, digest in old.items():
        if current.get(name) != digest:
            raise ValueError(f"historical migration changed or missing: {name}")
    maximum = max(int(name.split("_")[0]) for name in old)
    for name in current.keys() - old.keys():
        if int(name.split("_")[0]) <= maximum:
            raise ValueError(f"new migration must follow historical versions: {name}")


def write_json(path, value):
    path.write_text(json.dumps(value, indent=2) + "\n")


def prepare(args):
    releases = json.loads(Path(args.releases).read_text())
    # gh api --slurp returns one list per page.
    if releases and isinstance(releases[0], list):
        releases = [item for page in releases for item in page]
    baseline = select_baseline(releases, args.ref)
    subprocess.run(["git", "merge-base", "--is-ancestor", baseline, "HEAD"], check=True)
    previous = git("rev-parse", f"{baseline}^{{commit}}")
    names = git("ls-tree", "--name-only", f"{baseline}:{MIGRATIONS}").splitlines()
    old = {name: hashlib.sha256(subprocess.check_output(
        ["git", "show", f"{baseline}:{MIGRATIONS}/{name}"])).hexdigest() for name in names}
    items = inventory(Path(MIGRATIONS))
    check_history(old, {item["file"]: item["sha256"] for item in items})
    source_version = max(int(name.split("_")[0]) for name in old)
    target = max(item["version"] for item in items)
    contract = {"format_version": 1, "source_revision": git("rev-parse", "HEAD"),
                "previous_release": baseline, "previous_revision": previous,
                "from_version": source_version, "to_version": target,
                "allowed_start_versions": sorted({0, source_version, target}),
                "expected_schema": {"version": target, "dirty": False},
                "migration_change": target != source_version, "migrations": items}
    write_json(Path(args.output), contract)


def validate_inputs(revision, evidence, images):
    if not re.fullmatch(r"[0-9a-f]{40}", revision):
        raise ValueError("release requires a full source revision")
    if evidence.get("result") != "success" or evidence.get("source_revision") != revision or not evidence.get("url"):
        raise ValueError("mandatory QA must pass for this exact source revision")
    checks = evidence.get("mandatory_checks", {})
    if set(checks) != MANDATORY or any(check.get("result") != "success" for check in checks.values()):
        raise ValueError("every mandatory QA job must be present and successful")
    if set(images) != {"api", "worker"}:
        raise ValueError("both API and worker image references are required")
    for image in images.values():
        if not re.fullmatch(r"sha256:[0-9a-f]{64}", image["digest"]):
            raise ValueError("image requires a registry manifest digest")
        if not re.fullmatch(r"ghcr\.io/[a-z0-9_./-]+:[a-zA-Z0-9_.-]+", image["tag"]):
            raise ValueError("image requires an explicit GHCR tag")


def package(args):
    contract = json.loads(Path(args.contract).read_text())
    evidence = json.loads(Path(args.evidence).read_text())
    images = json.loads(Path(args.images).read_text())
    validate_inputs(git("rev-parse", "HEAD"), evidence, images)
    if contract["source_revision"] != evidence["source_revision"]:
        raise ValueError("migration contract is from another revision")
    if inventory(Path(MIGRATIONS)) != contract["migrations"]:
        raise ValueError("migrations differ from the tested contract")
    version = re.fullmatch(r"v(\d+\.\d+\.\d+(?:-rc\d+)?)", args.tag)
    if not version or git("rev-parse", f"{args.tag}^{{commit}}") != contract["source_revision"]:
        raise ValueError("release tag must identify the tested source revision")
    for image in images.values():
        if image["tag"].rsplit(":", 1)[1] != version[1]:
            raise ValueError("image tag must match the release version")
        image["reference"] = image["tag"].rsplit(":", 1)[0] + "@" + image["digest"]
    destination = Path(args.output)
    destination.mkdir(parents=True, exist_ok=False)
    shutil.copytree(MIGRATIONS, destination / "migrations")
    shutil.copytree("release", destination / "instructions")
    shutil.copyfile("docs/RELEASING.md", destination / "RELEASING.md")
    shutil.copyfile(args.evidence, destination / "qa.json")
    # Enumerate every file deployed or executed; the archive is additionally attested.
    files = {str(p.relative_to(destination)): hashlib.sha256(p.read_bytes()).hexdigest()
             for p in sorted(destination.rglob("*")) if p.is_file()}
    manifest = {**contract, "release": args.tag, "images": images,
                "documentation": f"https://github.com/{args.repository}/blob/{contract['source_revision']}/docs/RELEASING.md",
                "release_notes": f"https://github.com/{args.repository}/releases/tag/{args.tag}",
                "application_prs": json.loads(Path(args.prs).read_text()),
                "qa": evidence, "files_sha256": files,
                "deployment": {"order": ["migration", "database-validation", "rolling-deployment"],
                               "migration_image": images["api"]["reference"],
                               "validation_image": images["api"]["reference"],
                               "migrate_command": ["/app/db-release", "migrate"],
                               "validate_command": ["/app/db-release", "validate"],
                               "failure_action": "stop; operator resolution/roll-forward; no automatic rollback"}}
    write_json(destination / "manifest.json", manifest)
    archive = destination.with_suffix(".tar.gz")
    with tarfile.open(archive, "w:gz") as bundle:
        bundle.add(destination, arcname="handoff")
    Path(str(archive) + ".sha256").write_text(hashlib.sha256(archive.read_bytes()).hexdigest() + "  " + archive.name + "\n")


def verify_image(args):
    root = Path(args.root)
    contract = json.loads(Path(args.contract).read_text())
    if json.loads((root / "release/contract.json").read_text()) != contract:
        raise ValueError("API image does not contain the tested migration contract")
    if inventory(root / "migrations") != contract["migrations"]:
        raise ValueError("API image migrations differ from the tested inventory")
    for path in Path("release").iterdir():
        if path.is_file() and path.name != "contract.json":
            if (root / "release" / path.name).read_bytes() != path.read_bytes():
                raise ValueError(f"API image instruction differs: {path.name}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    prep = commands.add_parser("prepare")
    prep.add_argument("--releases", required=True)
    prep.add_argument("--ref", required=True)
    prep.add_argument("--output", required=True)
    pkg = commands.add_parser("package")
    for flag in ("contract", "evidence", "images", "tag", "repository", "prs", "output"):
        pkg.add_argument("--" + flag, required=True)
    verify = commands.add_parser("verify-image")
    verify.add_argument("--root", required=True)
    verify.add_argument("--contract", required=True)
    args = parser.parse_args()
    try:
        {"prepare": prepare, "package": package, "verify-image": verify_image}[args.command](args)
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        parser.exit(1, f"release handoff: {error}\n")


if __name__ == "__main__":
    main()
