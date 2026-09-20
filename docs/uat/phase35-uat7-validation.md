# Phase 35A UAT7 validation

Source candidate includes the Home Devices Comfortable/Compact presentation modes and the generated embedded frontend bundle.

Required release gates:
- frontend dependency audit
- Host/Home UX semantic regression
- embedded frontend bundle verification
- backend tests
- race tests
- go vet
- backend build
- Docker build, Compose validation, and smoke test
- release artifact dry run

No Phase 35.8 Proxmox API credentials or automatic sync are included.
No merge to main is authorized by this document.
