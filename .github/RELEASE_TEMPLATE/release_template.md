# API/worker release checklist

Follow [docs/RELEASING.md](../../docs/RELEASING.md).

- [ ] Mandatory Go/security, cross-build, frontend and PostgreSQL compatibility checks pass on the exact release revision.
- [ ] Release PR documents schema changes or explicitly states no schema change.
- [ ] N−1 remains compatible during migration and N−1/N rolling overlap; affected API/worker fixture assertions are updated.
- [ ] Applied migrations are immutable; expand/transition/contract and consumer retirement requirements are documented.
- [ ] Backfills account for concurrent writes; meaningful locking/data-volume/performance evidence is linked from development, CI or pre-production.
- [ ] Finite read-only SQL invariants and execution limits cover release-specific schema/data requirements.
- [ ] Tagged release includes attested `release-handoff.tar.gz`, checksum, API/worker digests, migration inventory, schema states, application PR/docs links and mandatory QA evidence.
- [ ] CD receives explicit references and instructions: migration → database validation → rolling deployment; stop on failure for operator resolution/roll-forward.

Creating a tag publishes artifacts and may trigger existing frontend delivery. Implementation or PR approval alone does not authorize a new release or deployment.
