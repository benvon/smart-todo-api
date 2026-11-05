# API and worker release contract

CI owns the release package and QA decision. CD receives explicit image references and associated instructions; it does not discover releases or grade QA evidence. This contract covers API, workers and PostgreSQL migrations. Frontend delivery remains separate.

## Mandatory QA before release creation

The tag-triggered release workflow calls this repository's quality workflow at the same revision. All five mandatory jobs must succeed: existing Go quality/security checks, cross-platform builds, existing frontend quality checks, PostgreSQL database compatibility, and the migration-safety workflow inherited from #105. `mandatory-qa` rejects missing, failed, skipped or cancelled results. Earlier PR success is insufficient: release QA runs again on the exact tagged revision.

The existing bot tagger is followed by an explicit dispatch of the gated release workflow at that tag. GitHub suppresses push-triggered runs for tags pushed using `GITHUB_TOKEN`, but permits `workflow_dispatch`; this keeps release creation working without another credential. See [GitHub's token event rules](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/trigger-a-workflow). Manual dispatch at a branch cannot publish a release. This dispatch targets only the API/worker release; the frontend workflow retains its existing tag-push trigger.

Database QA selects the highest published stable release below the candidate version (or latest stable for main/PR builds), verifies ancestry, and records its tag and commit. Release candidates use the previous stable baseline. This requires no prior deployment or production health check. A published stable baseline must exist; changing the support window requires a maintainer policy change.

The initial database policy exercises:

- Empty PostgreSQL bootstrap, N−1 → N migration, clean schema/version and SQL invariants, and replay with no pending changes.
- N−1 HTTP handlers writing during migration, then N−1 and N handlers/repositories/task processors concurrently using one database. Each worker processes the other revision's API writes. Both fixtures compile against actual release source; external AI is deterministic. This tests application/database compatibility, not RabbitMQ delivery or external providers.
- An additive column migration fixture even when the candidate schema is unchanged.
- Dirty/unsupported/malformed schema rejection, failed migrations, false/missing/multiple validation results, read-only enforcement, and statement, lock and overall deadlines.
- A reproducible 1,000-row data-volume smoke fixture. This is not a production-capacity benchmark. Changes with meaningful locking, backfill or performance impact require maintainer-selected representative-volume/locking tests in development, CI or pre-production, linked from the release PR/documentation.

This is the starting policy; maintainers extend it as the application evolves. There is no independent QA approval service.

## Tagged release package

After mandatory QA succeeds, GoReleaser builds existing artifacts and creates a draft. CI adds `release-handoff.tar.gz` and its SHA-256 file, attests the archive using GitHub build provenance, then publishes the release. Failure stops publication; operators resolve incomplete drafts explicitly before retrying. The frontend workflow only attaches its package to an already published release.

The archive contains:

- `manifest.json`: format version, tag/source commit, previous stable tag/commit, API and worker tags/registry manifest digests, migration inventory/checksums, starting/target schema, no-change indicator, runtime commands, deployment order, failure instructions, QA evidence, application PR/documentation/release-note links.
- `migrations/`: every immutable up/down SQL file. Historical files support bootstrap; down files do not authorize automated rollback.
- `instructions/contract.json`, `instructions/validation.json` and release-specific SQL invariants: the same instructions embedded in the API image.
- `qa.json` and `RELEASING.md`: maintainer-owned results, exact workflow run/attempt link and operating contract.

Manifest SHA-256 entries cover every instruction, migration, QA file and document; the attested archive binds the manifest to the source workflow. Registry manifest digests are used instead of image configuration hashes. Deploy the `reference` fields (`repository@sha256:…`). Existing GoReleaser binary artifacts/checksums remain in place. Verify the archive checksum and artifact attestation against the release workflow/source revision before consuming instructions, for example `gh attestation verify release-handoff.tar.gz --repo benvon/smart-todo-api`.

## Database and GitOps handoff

Requestor supplies API/worker digest references and matching package → deployment PR → review/merge → Argo migration → bounded database validation → rolling deployment. This repository supplies the interface; it does not implement CD, Argo hooks or deployment PR creation.

Run both database steps in the packaged API image by digest, injecting `DATABASE_URL` through the deployment's secret mechanism:

1. `/app/db-release migrate`
2. `/app/db-release validate`
3. Roll out supplied API/worker images only after both steps succeed.

`/app/run_migrations.sh` retains #105's independent guarded migration entry point unchanged. The release-specific migration command calls that same packaged guarded helper using its `up` interface, with the connection URL supplied only through the environment; it additionally enforces the release schema contract. Use `/app/db-release migrate` for the release handoff. Use separate ordered jobs: migration success alone never authorizes rollout. Give each orchestration job a slightly longer deadline than its process (for example 150 seconds), avoid automatic migration retries and retain failure diagnostics. Each release command has a 120-second overall deadline with up to 10 seconds for guarded-child cancellation and cleanup. Validation uses a 5-second connection timeout, 10-second statement timeout and 2-second lock timeout. The migration helper retains its own 90-second migration/session budget, 2-second connection limit and guarded cancellation/session verification from #105; its upstream migration lock wait is bounded by that helper deadline. The legacy shell entry point retains its separate 300-second job budget. Configuration validation caps limits to prevent unbounded instructions.

Migration accepts only a truly empty database, recorded N−1 version or clean target version. It rejects dirty/unsupported states, never forces a version and never runs down migrations. Bootstrap applies immutable history; upgrades apply pending files. Equal starting/target versions indicate no schema change: the runtime still checks the actual clean target and runs all validation. Missing version tracking in a nonempty database requires operator resolution.

Validation requires the exact target version with `dirty=false`, then runs a finite SQL invariant list in read-only transactions. Each file returns exactly one row containing one true boolean. Use catalog or indexed bounded checks. No production sampling, soak/load test, rehearsal or application readiness grading belongs here. Existing application instances remain live throughout.

Failures stop CD for operator resolution/roll-forward. Preserve the failed step, expected/found schema and invariant name. Never print credentials, force-clean a dirty migration, automatically roll back images or require prior deployment success.

## Obligations for future changes

Support N−1 → N. N's migrations must work with N−1 API/workers actively reading/writing, then both revisions must coexist on the migrated schema. Design N for meaningful N+1 overlap; unknown future code cannot be tested in advance.

Never modify, rename or delete applied migrations. CI compares names/checksums against the previous stable release and rejects new files inserted behind the historical maximum. Use expand → transition → contract: add compatible structures, transition readers/writers and backfill accounting for concurrent writes, then clean up only after affected consumers and rollback support retire. A version bump does not prove retirement. Split breaking changes across releases and record retirement conditions in the application PR.

Update the shared compatibility fixture and release-specific SQL invariants for affected behavior. The fixture compiles against N−1; do not patch old application code or remove assertions to hide incompatibility. Document schema effects, online overlap, backfill/concurrency behavior, limits and relevant load/locking evidence in the release PR. State explicitly when there is no schema change.

Keep workflow changes repository-local until stabilized. Go/dependency modernization, new releases and live deployments remain separate scope. For local API image builds, first run `scripts/ci_release.py prepare` against published-release metadata to generate ignored `release/contract.json`; missing instructions should fail the build instead of producing an unbound migration image.

The migration helper's existing option restrictions and pre-release configuration-classification obligation remain in force; see [tools/migrate/README.md](../tools/migrate/README.md). Do not promote a release against unsupported runtime options, weaken cancellation/session verification, or print driver messages to diagnose failures. This integration does not authorize a production configuration change. PostgreSQL overlap QA relocates only the helper's fixed migration-source path in a temporary test copy; the production helper has no new path override. The migration-safety workflow additionally tests the actual packaged helper and unchanged shell entry point.
