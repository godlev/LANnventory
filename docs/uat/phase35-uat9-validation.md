# Phase 35A UAT9 validation

Validated source candidate fixes Home Proxmox VM/LXC totals for lowercase current Host MAC representations.

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

Regression focus:
- lowercase Host MAC + canonical workload owner MAC still resolves hypervisor profile
- Home workload totals render for Proxmox
- mock backend exposes workload summaries

No Phase 35.8 Proxmox API credentials or automatic sync are included.
No merge to main is authorized by this document.
