#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${script_dir}/lannventory.sh"

fail() {
  printf 'Installer helper test failed: %s\n' "$*" >&2
  exit 1
}

assert_success() {
  "$@" || fail "expected success: $*"
}

assert_failure() {
  if ( "$@" ) >/dev/null 2>&1; then
    fail "expected failure: $*"
  fi
}

assert_equal() {
  local got="$1"
  local want="$2"
  local label="$3"

  [[ "$got" == "$want" ]] || fail "${label}: got '${got}', want '${want}'"
}

assert_success validate_ipv4 "0.0.0.0"
assert_success validate_ipv4 "192.168.1.1"
assert_success validate_ipv4 "255.255.255.255"
assert_failure validate_ipv4 "999.999.999.999"
assert_failure validate_ipv4 "192.168.1"
assert_failure validate_ipv4 "192.168.1.1.1"
assert_failure validate_ipv4 "192.168.1.a"

assert_success validate_ipv4_cidr "192.168.1.50/24"
assert_success validate_ipv4_cidr "10.0.0.1/32"
assert_success validate_ipv4_cidr "10.0.0.1/0"
assert_failure validate_ipv4_cidr "192.168.1.50/33"
assert_failure validate_ipv4_cidr "999.168.1.50/24"
assert_failure validate_ipv4_cidr "192.168.1.50"

filtered_bridges="$(printf '%s\n' fwbr100 fwln100 fwpr100 vmbr1 vmbr0 br-lan | filter_valid_bridges | paste -sd ' ' -)"
assert_equal "$filtered_bridges" "br-lan vmbr0 vmbr1" "bridge filtering"

BRIDGES=(br-lan vmbr0 vmbr1)
assert_equal "$(default_bridge)" "vmbr0" "vmbr0 default preference"

BRIDGES=(br-lan vmbr1)
assert_equal "$(default_bridge)" "br-lan" "first bridge fallback"

expected_url="https://github.com/godlev/LANnventory/releases/download/v0.1.0-beta.1/lannventory_0.1.0-beta.1_linux_amd64.deb"
assert_equal "$(package_url_for_arch amd64)" "$expected_url" "amd64 package URL"
assert_equal "$(package_url_for_arch x86_64)" "$expected_url" "x86_64 package URL"
assert_failure package_url_for_arch arm64

printf 'LANnventory Proxmox installer helper tests passed\n'
