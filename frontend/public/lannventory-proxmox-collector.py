#!/usr/bin/env python3
"""Read-only Proxmox inventory collector for LANnventory Phase 35.

The collector prints a normalized JSON snapshot to stdout. It does not contact
LANnventory or the Internet, does not modify Proxmox, and never emits raw guest
configuration. Network metadata is extracted through an explicit allowlist.
"""

from __future__ import annotations

import argparse
import datetime as dt
import ipaddress
import json
import os
import re
import shutil
import subprocess
import sys
from typing import Iterable

SCHEMA_VERSION = 1
COLLECTOR_VERSION = "1.0.1"
SOURCE = "script-import"

QEMU_NETWORK_PATTERN = r"^(net|ipconfig)[0-9]+:"
LXC_NETWORK_PATTERN = r"^net[0-9]+:"
MAC_PATTERN = re.compile(r"(?i)(?:[0-9a-f]{2}:){5}[0-9a-f]{2}")


class CollectionError(RuntimeError):
    pass


def utc_now() -> str:
    return dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")


def run_command(args: list[str], label: str) -> str:
    try:
        result = subprocess.run(args, check=False, capture_output=True, text=True)
    except OSError as exc:
        raise CollectionError(f"{label} unavailable") from exc
    if result.returncode != 0:
        raise CollectionError(f"{label} failed")
    return result.stdout


def read_allowlisted_config(path: str, pattern: str, label: str) -> list[str]:
    """Return only matching allowlisted lines from the active config section.

    Proxmox stores snapshot sections after the active configuration using
    [snapshot-name] headers. The collector must stop before the first such
    section, otherwise historical netN/ipconfigN values can be mistaken for the
    current guest network configuration.

    awk reads the source file, exits at the first section header, and only
    matching allowlisted lines cross the process boundary into the collector.
    Raw config, disks, passwords, tokens and user-data keys are never captured
    by this Python process or emitted in the snapshot.
    """
    awk_program = f'/^[[:space:]]*\\[/ {{ exit }} /{pattern}/ {{ print }}'
    try:
        result = subprocess.run(
            ["awk", awk_program, path],
            check=False,
            capture_output=True,
            text=True,
        )
    except OSError as exc:
        raise CollectionError(f"{label} network metadata unavailable") from exc

    if result.returncode != 0:
        raise CollectionError(f"{label} network metadata unavailable")
    return [line.strip() for line in result.stdout.splitlines() if line.strip()]


def parse_comma_values(value: str) -> dict[str, str]:
    result: dict[str, str] = {}
    for token in value.split(","):
        token = token.strip()
        if "=" not in token:
            continue
        key, item = token.split("=", 1)
        key = key.strip().lower()
        if key:
            result[key] = item.strip()
    return result


def normalize_mac(value: str) -> str:
    match = MAC_PATTERN.search(value)
    return match.group(0).upper() if match else ""


def normalize_status(value: str) -> str:
    value = value.strip().lower()
    if value == "running":
        return "running"
    if value == "stopped":
        return "stopped"
    return "unknown"


def configured_address(value: str) -> str:
    value = value.strip()
    if value.lower() in {"", "dhcp", "manual", "auto"}:
        return ""
    return value


def configured_network(address: str) -> str:
    if not address:
        return ""
    try:
        return str(ipaddress.ip_interface(address).network)
    except ValueError:
        return ""


def parse_guest_list(output: str, workload_type: str) -> list[dict[str, str]]:
    workloads: list[dict[str, str]] = []
    for raw in output.splitlines():
        line = raw.strip()
        if not line or not line[0].isdigit():
            continue
        fields = line.split()
        if len(fields) < 2:
            continue

        native_id = fields[0]
        if workload_type == "vm":
            name = fields[1] if len(fields) >= 2 else ""
            status = fields[2] if len(fields) >= 3 else ""
        else:
            status = fields[1] if len(fields) >= 2 else ""
            name = fields[-1] if len(fields) >= 3 else ""

        workloads.append(
            {
                "nativeId": native_id,
                "workloadType": workload_type,
                "name": name,
                "status": normalize_status(status),
            }
        )
    return workloads


def parse_qemu_interfaces(lines: Iterable[str]) -> list[dict[str, str]]:
    nets: dict[int, dict[str, str]] = {}
    ipconfigs: dict[int, dict[str, str]] = {}

    for line in lines:
        if ":" not in line:
            continue
        key, value = line.split(":", 1)
        key = key.strip().lower()
        value = value.strip()

        net_match = re.fullmatch(r"net([0-9]+)", key)
        if net_match:
            index = int(net_match.group(1))
            values = parse_comma_values(value)
            nets[index] = {
                "name": key,
                "mac": normalize_mac(value),
                "bridge": values.get("bridge", ""),
                "vlanTag": values.get("tag", ""),
            }
            continue

        ip_match = re.fullmatch(r"ipconfig([0-9]+)", key)
        if ip_match:
            ipconfigs[int(ip_match.group(1))] = parse_comma_values(value)

    interfaces: list[dict[str, str]] = []
    for index in sorted(set(nets) | set(ipconfigs)):
        item = nets.get(index, {"name": f"net{index}", "mac": "", "bridge": "", "vlanTag": ""})
        ip_values = ipconfigs.get(index, {})
        address = configured_address(ip_values.get("ip", ""))
        if not address:
            address = configured_address(ip_values.get("ip6", ""))
        interfaces.append(
            {
                "name": item["name"],
                "mac": item["mac"],
                "bridge": item["bridge"],
                "vlanTag": item["vlanTag"],
                "configuredAddress": address,
                "configuredNetwork": configured_network(address),
            }
        )
    return interfaces


def parse_lxc_interfaces(lines: Iterable[str]) -> list[dict[str, str]]:
    interfaces_by_key: dict[str, dict[str, str]] = {}
    for line in lines:
        if ":" not in line:
            continue
        key, value = line.split(":", 1)
        key = key.strip().lower()
        if not re.fullmatch(r"net[0-9]+", key):
            continue
        values = parse_comma_values(value)
        address = configured_address(values.get("ip", ""))
        if not address:
            address = configured_address(values.get("ip6", ""))
        interfaces_by_key[key] = {
            "name": values.get("name", key),
            "mac": normalize_mac(values.get("hwaddr", value)),
            "bridge": values.get("bridge", ""),
            "vlanTag": values.get("tag", ""),
            "configuredAddress": address,
            "configuredNetwork": configured_network(address),
        }
    return sorted(interfaces_by_key.values(), key=lambda item: item["name"])


def collect_cluster_name() -> str:
    path = "/etc/pve/corosync.conf"
    if not os.path.exists(path):
        return ""
    lines = read_allowlisted_config(path, r"^[[:space:]]*cluster_name[[:space:]]*:", "cluster")
    if not lines:
        return ""
    return lines[0].split(":", 1)[1].strip()


def collect_node() -> dict[str, str]:
    hostname = run_command(["hostname", "-s"], "hostname").strip()
    raw_version = run_command(["pveversion"], "pveversion").strip().splitlines()
    version = raw_version[0].split()[0] if raw_version else ""
    if not hostname or not version:
        raise CollectionError("node identity incomplete")
    return {
        "hostname": hostname,
        "pveVersion": version,
        "clusterName": collect_cluster_name(),
        "status": "online",
    }


def collect_interfaces(workload_type: str, native_id: str) -> list[dict[str, str]]:
    if workload_type == "vm":
        path = f"/etc/pve/qemu-server/{native_id}.conf"
        lines = read_allowlisted_config(path, QEMU_NETWORK_PATTERN, f"vm {native_id}")
        return parse_qemu_interfaces(lines)

    path = f"/etc/pve/lxc/{native_id}.conf"
    lines = read_allowlisted_config(path, LXC_NETWORK_PATTERN, f"container {native_id}")
    return parse_lxc_interfaces(lines)


def collect_workload_type(command: list[str], workload_type: str, errors: list[str]) -> list[dict[str, object]]:
    label = "qemu guest list" if workload_type == "vm" else "lxc guest list"
    try:
        output = run_command(command, label)
    except CollectionError:
        errors.append(f"{label} unavailable")
        return []

    workloads: list[dict[str, object]] = []
    for item in parse_guest_list(output, workload_type):
        try:
            interfaces = collect_interfaces(workload_type, item["nativeId"])
        except CollectionError:
            errors.append(f"{workload_type} {item['nativeId']}: network metadata unavailable")
            interfaces = []

        workloads.append(
            {
                **item,
                "interfaces": interfaces,
            }
        )
    return workloads


def preflight() -> None:
    required = ["hostname", "pveversion", "qm", "pct", "awk"]
    missing = [command for command in required if shutil.which(command) is None]
    if missing:
        raise CollectionError("required Proxmox commands unavailable: " + ", ".join(sorted(missing)))


def collect_snapshot() -> dict[str, object]:
    errors: list[str] = []
    try:
        node = collect_node()
    except CollectionError:
        node = {"hostname": "", "pveVersion": "", "clusterName": "", "status": "unknown"}
        errors.append("node identity unavailable")

    workloads: list[dict[str, object]] = []
    workloads.extend(collect_workload_type(["qm", "list"], "vm", errors))
    workloads.extend(collect_workload_type(["pct", "list"], "container", errors))
    workloads.sort(key=lambda item: (str(item["workloadType"]), int(str(item["nativeId"]))))

    return {
        "schemaVersion": SCHEMA_VERSION,
        "collectorVersion": COLLECTOR_VERSION,
        "source": SOURCE,
        "collectedAt": utc_now(),
        "complete": len(errors) == 0,
        "collectionErrors": errors,
        "node": node,
        "workloads": workloads,
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Collect an allowlisted, read-only Proxmox inventory snapshot for LANnventory."
    )
    parser.add_argument("--compact", action="store_true", help="emit compact JSON instead of inspectable indented JSON")
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    try:
        preflight()
        snapshot = collect_snapshot()
    except CollectionError as exc:
        snapshot = {
            "schemaVersion": SCHEMA_VERSION,
            "collectorVersion": COLLECTOR_VERSION,
            "source": SOURCE,
            "collectedAt": utc_now(),
            "complete": False,
            "collectionErrors": [str(exc)],
            "node": {"hostname": "", "pveVersion": "", "clusterName": "", "status": "unknown"},
            "workloads": [],
        }

    if args.compact:
        print(json.dumps(snapshot, separators=(",", ":"), sort_keys=True))
    else:
        print(json.dumps(snapshot, indent=2, sort_keys=True))
    return 0 if snapshot["complete"] else 2


if __name__ == "__main__":
    sys.exit(main())
