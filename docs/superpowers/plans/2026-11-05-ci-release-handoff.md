# CI release handoff implementation plan

**Goal:** Implement issue #106's clarified CI contract without publishing a release or changing live services.

**Architecture:** Reuse repository-local quality jobs from tag releases so every mandatory check runs on the release revision before publication. Publish an attested handoff archive with API/worker registry digests, migration inventory, bounded validation SQL, and links to successful QA jobs. PostgreSQL tests exercise previous-release application code during migration and both revisions during overlap.

**Spec:** https://github.com/benvon/smart-todo-api/issues/106

**Constraints:** N−1 → N; forward-compatible design where meaningful; immutable historical migrations; no shared workflow changes, dependency modernization, live deployment, automatic rollback, or new application release. Existing frontend checks remain mandatory; frontend artifacts are outside the handoff.

## Tasks

- [x] Add tested release tooling: select the previous published stable ancestor, reject changed/deleted historical migrations, inventory migration checksums and expected versions, and generate the release handoff. Fail closed for missing/unsuccessful QA, malformed digests, or incomplete inputs.
- [x] Add bounded runtime migration/validation commands, using the server image and a pinned migration tool. Check clean versions, allow only the supported starting or target version, handle no-change, and stop with diagnostics on errors. Package release-specific read-only SQL invariants.
- [x] Add real PostgreSQL QA: fresh database, N−1 upgrade and replay/no-change, old code active through migration, both revisions concurrently after migration, additive change fixture, negative validation, dirty/unsupported schema, and migration immutability. Use real HTTP handlers and worker processors with deterministic AI and an isolated database.
- [x] Gate release creation on repository-local mandatory quality jobs at the exact tag revision; package and attest handoff artifacts before publishing the draft. Prevent the frontend workflow from independently creating a release.
- [x] Update release/contributor/agent guidance; run meaningful tests and workflow/configuration checks; review failure and publication paths. Record any unavailable checks.

## Review focus

- No failed, skipped, cancelled, or missing QA job may reach release creation.
- Published images, migrations, SQL, source revision, and QA evidence must identify one release.
- Baseline is a published stable ancestor, not an arbitrary tag or a production-health prerequisite.
- Applied migration removal/rename/modification, dirty schemas and unsupported starting versions must fail.
- Runtime migration and validation have finite connection, lock, statement and overall deadlines; logs must not expose database credentials.

## Verification

Run Python unit tests, build the compatibility fixture against both revisions, run PostgreSQL integration QA in an isolated local container, run Go tests and GoReleaser/workflow validation where tools are available. No push, tag, release, deployment or live database operations are part of verification.

## Verification results

- Python release-policy/archive/image-binding tests: 5 passed.
- Go tests, race tests for application packages/new command, and 20 cross-platform command builds passed.
- PostgreSQL 16 QA passed with published v4.1.1 baseline: bootstrap, distinct no-change/release/additive overlap, bidirectional API/worker processing, dirty/unsupported/missing/multiple schema records, read-only validation, statement/lock/overall limits and failed migration stop.
- Actionlint passed all four affected workflows; GoReleaser configuration validated.
- First-party Go lint passed using available golangci-lint 2.13.2. Whole-tree local lint additionally sees third-party Go code under installed web/node_modules and reports issues there; CI Go checkout has no node_modules. The pinned CI linter and remote security/QA workflow have not been run locally.
- Final linux/amd64 API and worker images built locally. Existing Docker builders required alignment from 1.25.9 to the already-declared go.mod 1.25.13; builds now preserve module files and pin the migration CLI. No application dependency changes.
- Independent review findings addressed: PR read permission and permanent no-change overlap coverage. Local bot-tag dispatch also reviewed; no outstanding actionable findings.
- Remote dispatch, release publication, GHCR publication, artifact attestation and deployment were deliberately not executed.
