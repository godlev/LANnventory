# Phase 35A UAT8 validation

Validated source candidate adds read-only Proxmox workload totals to the Home Devices table.

Required release gates:
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

Count semantics:
- current non-retired workloads only
- running and stopped workloads both count
- counts are based on full workload inventory, not only linked Hosts

No Phase 35.8 Proxmox API credentials or automatic sync are included.
No merge to main is authorized by this document.
