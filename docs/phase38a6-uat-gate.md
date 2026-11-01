# Phase 38A.6 — Relationship Foundation UAT gate

Development branch: `phase38a-relationship-foundation`. Do not merge into `main` or publish a stable Beta before approval.

## Automated gate

- [x] Packaging Check, frontend build/security audit, backend tests and Docker build: GitHub Actions run 41347148395, commit a4ae0a2103ded49f5e9e65cf0c4a8c0940ab1519.
- [x] Database migration repeatability and unique identity: `TestRelationshipMigrationCreatesUniqueIndexesAndIsIdempotent`.
- [x] Manual assertion persistence across `Close` / `StartErr`: `TestManualRelationshipPersistenceUniqueAndProvenanceProtected`.
- [x] Host deletion cleanup and transaction rollback: `TestHostDeletionPrunesBothDirectionsButPreservesUnrelated`.
- [x] Logical backup format v6 and legacy v5 decoding: `TestLogicalBackupV6IncludesManualAssertionsAndLegacyJSONRemainsDecodable`.
- [x] Graph and API behavior: `api_relationship_graph_test.go`, `api_relationship_regression_38a6_test.go`.
- [ ] Real CT129 upgrade and live UAT (not represented by CI).

**Backup caveat:** LANnventory supports logical export but does not currently provide a logical restore importer. Preserve a full CT/snapshot backup before deployment.

## Live UAT: CT129

1. Record installed application version, CT ID, and currently working Proxmox inventory; take and verify a Proxmox CT backup/snapshot **before** upgrading.
2. Install an explicit 38A UAT release only after its packaging/publishing checks pass. Do not install an unreviewed development artifact.
3. Verify that Host Inventory → Relationships renders manual and read-only Proxmox-derived relationships, preserves VM/LXC typing, and shows direction and provenance.
4. Create a manual `manages` or `depends_on` relation between two existing LAN Hosts; confirm duplicate create is blocked and reverse direction remains distinct.
5. Edit its note; refresh the page; restart the LANnventory service; verify relationship identity/note remain unchanged.
6. Verify that Proxmox-derived `hosts` / `matches_host` links cannot be edited/deleted from the relationship card; sync Proxmox and ensure no duplicate manual assertions appear.
7. Check missing/stale related Hosts and graph truncation/limit UI. Do **not** delete real inventory records merely to test cleanup; deletion is exercised by automated regression tests.
8. Confirm other Host/Settings/Proxmox workflows still work. Record screenshots, version and observed defects.
9. If failure occurs, stop deployment; restore the full CT backup following the operator's verified procedure, not a logical JSON export.

## Exit criteria

CI green; UAT release installed and exercised; relationship persistence and source ownership verified; no regressions in existing inventory; explicit user approval before any `main` merge.
