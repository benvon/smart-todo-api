"""Release inputs fail closed before publication; no network required."""
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock
import argparse
import os
import tarfile
import shutil
import subprocess
import re

SPEC = importlib.util.spec_from_file_location("ci_release", Path(__file__).with_name("ci_release.py"))
release = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(release)


class ReleaseTests(unittest.TestCase):
    def test_release_preparation_keeps_checkout_clean(self):
        root = Path(__file__).resolve().parents[1]
        workflow = (root / ".github/workflows/release.yml").read_text()
        download = workflow.split("uses: actions/download-artifact@v4", 1)[1]
        download_path = re.search(r"path: (.+)", download).group(1)
        preparation = workflow.split("name: Verify tested revision and prepare image instructions", 1)[1]
        commands = preparation.split("run: |\n", 1)[1].split("      - uses:", 1)[0]
        commands = "\n".join(line[10:] for line in commands.splitlines())
        with tempfile.TemporaryDirectory() as directory:
            temporary = Path(directory)
            checkout = temporary / "checkout"
            subprocess.run(["git", "clone", "--quiet", "--shared", str(root), str(checkout)], check=True)
            revision = subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=checkout, text=True).strip()
            runner_temp = temporary / "runner temp"
            github_env = temporary / "github-env"
            staging = workflow.split("name: Set release staging directory", 1)[1].split("\n", 2)[1]
            staging_command = staging.split("run: ", 1)[1]
            subprocess.run(["bash", "-c", staging_command], check=True,
                           env=dict(os.environ, RUNNER_TEMP=str(runner_temp), GITHUB_ENV=str(github_env)))
            qa_directory = Path(github_env.read_text().strip().split("=", 1)[1])
            destination = Path(download_path.replace("${{ env.RELEASE_QA_DIR }}", str(qa_directory)).replace("${{ runner.temp }}", str(runner_temp)))
            if not destination.is_absolute():
                destination = checkout / destination
            destination.mkdir(parents=True)
            (destination / "qa.json").write_text(json.dumps({"source_revision": revision, "result": "success"}))
            (destination / "contract.json").write_text('{"tested": true}')
            tools = temporary / "tools"
            tools.mkdir()
            gh = tools / "gh"
            gh.write_text("#!/bin/sh\nexit 1\n")  # No existing release; never call GitHub.
            gh.chmod(0o755)
            environment = dict(os.environ, RUNNER_TEMP=str(runner_temp), RELEASE_QA_DIR=str(qa_directory),
                               RELEASE_TAG="v4.2.1", PATH=str(tools) + os.pathsep + os.environ["PATH"])
            subprocess.run(["bash", "-c", commands], cwd=checkout, env=environment, check=True)
            self.assertEqual((checkout / "release/contract.json").read_text(), '{"tested": true}')
            status = subprocess.check_output(["git", "status", "--porcelain"], cwd=checkout, text=True)
            self.assertEqual(status, "", "Release preparation must preserve GoReleaser's clean-tree requirement")

    def test_baseline_uses_previous_published_stable(self):
        releases = [
            {"tag_name": "v4.2.0", "draft": False, "prerelease": False},
            {"tag_name": "v4.1.1", "draft": False, "prerelease": False},
            {"tag_name": "v4.3.0-rc1", "draft": False, "prerelease": True},
            {"tag_name": "v5.0.0", "draft": True, "prerelease": False},
        ]
        self.assertEqual(release.select_baseline(releases, "v4.2.0"), "v4.1.1")
        self.assertEqual(release.select_baseline(releases, "main"), "v4.2.0")
        with self.assertRaises(ValueError):
            release.select_baseline([], "main")

    def test_historical_migrations_are_immutable_and_append_only(self):
        old = {"000001_initial.up.sql": "a", "000001_initial.down.sql": "b"}
        release.check_history(old, dict(old))
        release.check_history(old, {**old, "000002_add.up.sql": "c", "000002_add.down.sql": "d"})
        for current in ({}, {**old, "000001_initial.up.sql": "changed"},
                        {**old, "000001_other.up.sql": "c"}):
            with self.subTest(current=current), self.assertRaises(ValueError):
                release.check_history(old, current)

    def test_inventory_requires_complete_unique_versions(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "000001_init.up.sql").write_text("SELECT 1;")
            with self.assertRaises(ValueError):
                release.inventory(root)
            (root / "000001_init.down.sql").write_text("SELECT 2;")
            items = release.inventory(root)
            self.assertEqual(len(items), 2)
            self.assertEqual(items[1]["sha256"], hashlib.sha256(b"SELECT 1;").hexdigest())
            (root / "000001_duplicate.up.sql").write_text("SELECT 3;")
            with self.assertRaises(ValueError):
                release.inventory(root)

    def test_package_requires_exact_revision_success_and_both_digests(self):
        sha = "a" * 40
        evidence = {"source_revision": sha, "result": "success", "url": "https://example.test/run"}
        evidence["mandatory_checks"] = {name: {"result": "success"} for name in release.MANDATORY}
        images = {name: {"tag": f"ghcr.io/benvon/test-{name}:1.2.3", "digest": "sha256:" + "b" * 64}
                  for name in ("api", "worker")}
        release.validate_inputs(sha, evidence, images)
        for result in ("failure", "skipped", "cancelled", "pending", None):
            with self.subTest(result=result), self.assertRaises(ValueError):
                release.validate_inputs(sha, {**evidence, "result": result}, images)
        with self.assertRaises(ValueError):
            release.validate_inputs("c" * 40, evidence, images)
        with self.assertRaises(ValueError):
            release.validate_inputs(sha, evidence, {"api": images["api"]})
        with self.assertRaises(ValueError):
            release.validate_inputs(sha, evidence, {**images, "worker": {"tag": "x", "digest": "latest"}})
        with self.assertRaises(ValueError):
            release.validate_inputs(sha, {**evidence, "mandatory_checks": {}}, images)
        for result in ("failure", "skipped", "cancelled", "pending"):
            checks = {**evidence["mandatory_checks"], "database-compatibility": {"result": result}}
            with self.subTest(check=result), self.assertRaises(ValueError):
                release.validate_inputs(sha, {**evidence, "mandatory_checks": checks}, images)

    def test_archive_binds_exact_migrations_images_instructions_and_qa(self):
        sha = "a" * 40
        with tempfile.TemporaryDirectory() as directory:
            previous = os.getcwd()
            os.chdir(directory)
            self.addCleanup(os.chdir, previous)
            migrations = Path(release.MIGRATIONS)
            migrations.mkdir(parents=True)
            for direction in ("up", "down"):
                (migrations / f"000001_init.{direction}.sql").write_text("SELECT true;")
            Path("release").mkdir()
            Path("docs").mkdir()
            Path("docs/RELEASING.md").write_text("Release contract")
            Path("release/check.sql").write_text("SELECT true;")
            contract = {"format_version": 1, "source_revision": sha, "previous_release": "v1.2.2",
                        "from_version": 1, "to_version": 1, "migration_change": False,
                        "migrations": release.inventory(migrations)}
            release.write_json(Path("release/contract.json"), contract)
            evidence = {"source_revision": sha, "result": "success", "url": "https://example.test/run",
                        "mandatory_checks": {name: {"result": "success"} for name in release.MANDATORY}}
            images = {name: {"tag": f"ghcr.io/benvon/test-{name}:1.2.3", "digest": "sha256:" + "b" * 64}
                      for name in ("api", "worker")}
            release.write_json(Path("qa.json"), evidence)
            release.write_json(Path("images.json"), images)
            release.write_json(Path("prs.json"), [{"number": 123, "url": "https://example.test/pr/123"}])
            shutil.copytree("release", "image/release")
            shutil.copytree(migrations, "image/migrations")
            image_args = argparse.Namespace(root="image", contract="release/contract.json")
            release.verify_image(image_args)
            Path("image/release/check.sql").write_text("SELECT false;")
            with self.assertRaises(ValueError):
                release.verify_image(image_args)
            Path("image/release/check.sql").write_text("SELECT true;")
            Path("image/migrations/000001_init.up.sql").write_text("changed")
            with self.assertRaises(ValueError):
                release.verify_image(image_args)
            args = argparse.Namespace(contract="release/contract.json", evidence="qa.json", images="images.json",
                                      prs="prs.json", tag="v1.2.3", repository="benvon/test", output="out/handoff")
            with mock.patch.object(release, "git", return_value=sha):
                release.package(args)
            archive_path = Path("out/handoff.tar.gz")
            checksum = Path(str(archive_path) + ".sha256").read_text().split()[0]
            self.assertEqual(checksum, hashlib.sha256(archive_path.read_bytes()).hexdigest())
            with tarfile.open(archive_path) as archive:
                manifest = json.load(archive.extractfile("handoff/manifest.json"))
                self.assertFalse(manifest["migration_change"])
                self.assertEqual(manifest["images"]["api"]["reference"], "ghcr.io/benvon/test-api@sha256:" + "b" * 64)
                for name, digest in manifest["files_sha256"].items():
                    self.assertEqual(hashlib.sha256(archive.extractfile("handoff/" + name).read()).hexdigest(), digest)
            # Do not leave cwd pointing at a directory being removed on Windows.
            os.chdir(previous)


if __name__ == "__main__":
    unittest.main()
