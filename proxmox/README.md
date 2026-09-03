# LANnventory Proxmox LXC Installer

This directory contains the official LANnventory Proxmox VE LXC installer.

Run this command from the Proxmox VE host shell as `root`:

```bash
bash -c "$(curl -fsSL https://raw.githubusercontent.com/godlev/LANnventory/main/proxmox/lannventory.sh)"
```

Do not run the installer inside an existing LXC. It creates a new Debian 13 unprivileged LXC and installs the official LANnventory release package.

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

## Installation Source

The current installer installs LANnventory `v0.1.0-beta.2` from the official GitHub release package:

```text
https://github.com/godlev/LANnventory/releases/download/v0.1.0-beta.2/lannventory_0.1.0-beta.2_linux_amd64.deb
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

This installer currently installs `v0.1.0-beta.2`. Future installer versions can update the release tag and package URL. Manual package upgrades should use a newer official LANnventory `.deb` release package and install it with `apt` inside the container so package dependencies remain managed by the OS.

## Notes About ARP Scanning In LXC

LANnventory requires `arp-scan` for real LAN discovery. A real test on Proxmox VE 9.2.10 with a Debian 13 amd64 unprivileged LXC confirmed that package installation, `lannventory.service`, `/api/health`, `/api/version`, `arp-scan` and LAN discovery work with `IFACES=eth0` and without extra capabilities, AppArmor relaxation or privileged mode.

This installer keeps the container unprivileged by default and does not loosen AppArmor or add broad capabilities automatically. If your Proxmox/LXC network policy differs and prevents ARP scanning from an unprivileged container, test and document the minimum required Proxmox setting for your environment before changing container security options.
