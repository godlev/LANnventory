# LANnventory Proxmox LXC Installer

This directory contains the official LANnventory Proxmox VE LXC installer.

Run this command from the Proxmox VE host shell as `root`:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/godlev/LANnventory/main/proxmox/lannventory.sh)"
```

Do not run the installer inside an existing LXC. It creates a new Debian 13 unprivileged LXC and installs the official LANnventory release package.


## Read-Only Inventory Collector

Phase 35 also includes a separate Proxmox inventory collector:

```bash
python3 proxmox/collect_inventory.py > lannventory-proxmox.json
```

The collector is independent from the LXC installer above. It does not install or update LANnventory, does not contact LANnventory, and does not transmit data to the Internet. It only prints a normalized JSON snapshot to standard output for the script-import workflow.

The snapshot contains only:

- schema/collector version, collection time and completeness state
- node hostname, Proxmox VE version, cluster name when present, and basic online status
- local QEMU VM and LXC IDs, names and running/stopped state
- explicitly allowlisted network identity: interface name, MAC, bridge, VLAN tag and configured address/network when available

Network configuration is extracted through an allowlist for `netN` and `ipconfigN` entries. Raw guest configuration is not emitted. Disk configuration, passwords, tokens, keys, arbitrary QEMU arguments, cloud-init/user data and other guest settings are not part of the snapshot.

If any required inventory section cannot be collected, the JSON is emitted with `"complete": false` and a non-secret summary in `collectionErrors`. Incomplete snapshots are intended to be rejected for destructive reconciliation such as workload retirement.

Use `--compact` only when compact JSON is preferred:

```bash
python3 proxmox/collect_inventory.py --compact
```

### Script Import Safety Flow

LANnventory does not persist collector output when it is first submitted. The import flow is deliberately two-step:

1. **Preview** strictly validates the snapshot contract and returns a deterministic diff.
2. The user reviews added, updated, unchanged, retired and conflicting workloads.
3. **Apply** requires an explicit confirmation and the preview token returned by the previous step.
4. LANnventory recomputes the preview against current persisted state. If inventory changed after preview, apply is rejected as stale.
5. A valid apply is committed in one database transaction.

Incomplete snapshots can be previewed so collection problems are visible, but they cannot be applied. A workload that already exists as manually maintained inventory is reported as a blocking source conflict rather than being overwritten.

Imported node hostname, Proxmox version, cluster and status are stored separately from the manually managed Hypervisor profile. Differences are shown as managed/imported conflicts, but imported data never overwrites manual Hypervisor fields.

The backend accepts at most 2 MiB per import request, rejects unknown JSON fields and validates schema version, source, RFC3339 collection time, VM/LXC identity, status, MAC addresses, VLAN tags, configured addresses and networks before generating a preview.

## What It Creates

Defaults:

- Hostname: `lannventory`
- OS: Debian 13
- Container type: unprivileged LXC
- CPU: 1 core
- RAM: 512 MB
- Swap: 512 MB
- Disk: 4 GB
- Network: DHCP
- Initial LANnventory interface: `eth0`
- Start on boot: yes
- Web port: `8840`

The installer detects available CT IDs, rootfs storages, template storages and Linux bridges, then asks you to confirm or change the important values before creating the container. Proxmox per-guest firewall bridges such as `fwbr*`, `fwpr*` and `fwln*` are filtered out of the normal bridge choices. If `vmbr0` exists, it is used as the default; otherwise the first valid non-firewall bridge is used. The installer never overwrites an existing CT or QEMU VMID.

The installer asks whether the new container should start automatically with Proxmox. The default is Yes.

## Installation Source

The current installer installs LANnventory `v0.1.0-beta.6` from the official GitHub release package:

```text
https://github.com/godlev/LANnventory/releases/download/v0.1.0-beta.6/lannventory_0.1.0-beta.6_linux_amd64.deb
```

No Docker setup is required. No source compilation is performed.

The Debian package declares the LANnventory runtime dependencies, including `arp-scan` and `tzdata`. The installer only installs basic bootstrap download requirements before installing the release package.

## What The Installer Does Not Do

- The installer script does not directly perform a LAN scan or execute `arp-scan`; it only verifies that `arp-scan` is available after package installation.
- It writes or updates the fresh LANnventory service configuration with `IFACES: "eth0"` before the installer starts the service, then restarts `lannventory.service` so the running process has loaded that configuration.
- After `lannventory.service` starts, LANnventory may begin its normal discovery using the configured interface.
- It does not build LANnventory from source.
- It does not publish or modify GitHub releases.
- It does not delete a partially created container if installation fails.

## After Installation

The installer enables and restarts:

```text
lannventory.service
```

It then checks these local endpoints from inside the container:

```text
http://127.0.0.1:8840/api/health
http://127.0.0.1:8840/api/version
```

Successful output includes the detected container IP and LANnventory URL:

```text
http://<container-ip>:8840
```

## Troubleshooting

Enter the container:

```bash
pct enter <CTID>
```

Check the service:

```bash
systemctl status lannventory
journalctl -u lannventory
```

From the Proxmox host, manage the container:

```bash
pct start <CTID>
pct stop <CTID>
```

If installation fails after the container is created, the installer leaves the container in place for inspection and prints the CT ID, failure stage, service status and recent journal output when available.

## Updating Later

This installer currently installs `v0.1.0-beta.3.1`. Future installer versions can update the release tag and package URL. Manual package upgrades should use a newer official LANnventory `.deb` release package and install it with `apt` inside the container so package dependencies remain managed by the OS.

## Notes About ARP Scanning In LXC

LANnventory requires `arp-scan` for real LAN discovery. A real test on Proxmox VE 9.2.10 with a Debian 13 amd64 unprivileged LXC confirmed that package installation, `lannventory.service`, `/api/health`, `/api/version`, `arp-scan` and LAN discovery work with `IFACES=eth0` and without extra capabilities, AppArmor relaxation or privileged mode.

This installer keeps the container unprivileged by default and does not loosen AppArmor or add broad capabilities automatically. If your Proxmox/LXC network policy differs and prevents ARP scanning from an unprivileged container, test and document the minimum required Proxmox setting for your environment before changing container security options.
