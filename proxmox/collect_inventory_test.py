import importlib.util
import json
import os
import pathlib
import tempfile
import unittest

MODULE_PATH = pathlib.Path(__file__).with_name("collect_inventory.py")
SPEC = importlib.util.spec_from_file_location("lannventory_proxmox_collector", MODULE_PATH)
collector = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(collector)


class CollectorParserTests(unittest.TestCase):
    def test_qemu_allowlisted_network_parsing(self):
        interfaces = collector.parse_qemu_interfaces([
            "net0: virtio=bc:24:11:a2:40:12,bridge=vmbr0,firewall=1,tag=20",
            "ipconfig0: ip=10.4.1.27/24,gw=10.4.1.1",
            "net1: e1000=AA:BB:CC:DD:EE:01,bridge=vmbr1",
            "ipconfig1: ip=dhcp",
        ])
        self.assertEqual(len(interfaces), 2)
        self.assertEqual(interfaces[0], {
            "name": "net0",
            "mac": "BC:24:11:A2:40:12",
            "bridge": "vmbr0",
            "vlanTag": "20",
            "configuredAddress": "10.4.1.27/24",
            "configuredNetwork": "10.4.1.0/24",
        })
        self.assertEqual(interfaces[1]["configuredAddress"], "")
        self.assertEqual(interfaces[1]["configuredNetwork"], "")

    def test_lxc_network_parsing(self):
        interfaces = collector.parse_lxc_interfaces([
            "net0: name=eth0,bridge=vmbr0,gw=10.4.1.1,hwaddr=AA:BB:CC:DD:EE:70,ip=10.4.1.70/24,type=veth,tag=30",
        ])
        self.assertEqual(interfaces, [{
            "name": "eth0",
            "mac": "AA:BB:CC:DD:EE:70",
            "bridge": "vmbr0",
            "vlanTag": "30",
            "configuredAddress": "10.4.1.70/24",
            "configuredNetwork": "10.4.1.0/24",
        }])

    def test_guest_lists_keep_only_identity_status(self):
        qemu = collector.parse_guest_list(
            " VMID NAME STATUS MEM(MB) BOOTDISK(GB) PID\n 119 media running 16384 100.00 1234\n",
            "vm",
        )
        lxc = collector.parse_guest_list(
            "VMID Status Lock Name\n127 running  yubal\n",
            "container",
        )
        self.assertEqual(qemu, [{"nativeId": "119", "workloadType": "vm", "name": "media", "status": "running"}])
        self.assertEqual(lxc, [{"nativeId": "127", "workloadType": "container", "name": "yubal", "status": "running"}])

    def test_config_reader_returns_only_explicit_allowlist(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = os.path.join(tmp, "119.conf")
            with open(path, "w", encoding="utf-8") as handle:
                handle.write("name: media\n")
                handle.write("cipassword: super-secret\n")
                handle.write("scsi0: local-lvm:vm-119-disk-0\n")
                handle.write("args: --private-value secret\n")
                handle.write("net0: virtio=BC:24:11:A2:40:12,bridge=vmbr0\n")
                handle.write("ipconfig0: ip=10.4.1.27/24,gw=10.4.1.1\n")

            lines = collector.read_allowlisted_config(path, collector.QEMU_NETWORK_PATTERN, "test vm")
            joined = "\n".join(lines)
            self.assertIn("net0:", joined)
            self.assertIn("ipconfig0:", joined)
            self.assertNotIn("super-secret", joined)
            self.assertNotIn("scsi0", joined)
            self.assertNotIn("args:", joined)
            self.assertNotIn("cipassword", joined)

    def test_collector_has_no_network_transfer_client(self):
        source = MODULE_PATH.read_text(encoding="utf-8").lower()
        for forbidden in [
            "import requests",
            "from requests",
            "import urllib",
            "from urllib",
            "import socket",
            "from socket",
            "curl ",
            "wget ",
            "http://",
            "https://",
        ]:
            self.assertNotIn(forbidden, source)

    def test_incomplete_snapshot_contract_is_explicit(self):
        snapshot = {
            "schemaVersion": collector.SCHEMA_VERSION,
            "collectorVersion": collector.COLLECTOR_VERSION,
            "source": collector.SOURCE,
            "collectedAt": collector.utc_now(),
            "complete": False,
            "collectionErrors": ["vm 119: network metadata unavailable"],
            "node": {"hostname": "pve", "pveVersion": "pve-manager/9.2.10", "clusterName": "", "status": "online"},
            "workloads": [],
        }
        payload = json.dumps(snapshot)
        self.assertIn('"complete": false', payload)
        self.assertNotIn("password", payload.lower())
        self.assertNotIn("token", payload.lower())


if __name__ == "__main__":
    unittest.main()
