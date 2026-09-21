# Phase 35 final beta.7 validation

This candidate finalizes the approved Phase 35A UAT work as v0.1.0-beta.7.

Required gates:
- frontend dependency audit
- Home/Host UX semantic regression
- embedded frontend bundle verification
- Swagger freshness
- backend tests
- race tests
- go vet
- backend build
- Docker build, Compose validation, and smoke test
- release artifact dry run
- updater selection regression from beta.6 and Phase 35 UAT releases to beta.7

Finalization cleanup:
- README current release points to beta.7
- completed one-shot UAT publish workflows are removed
- beta.7 curated release notes are present

Phase 35.8 Proxmox API credentials/automatic sync is not included in beta.7.
