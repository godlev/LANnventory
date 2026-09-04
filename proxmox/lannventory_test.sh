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

assert_file_contains() {
  local file="$1"
  local pattern="$2"
  local label="$3"

  grep -Fq -- "$pattern" "$file" || fail "${label}: missing '${pattern}'"
}

assert_file_not_contains() {
  local file="$1"
  local pattern="$2"
  local label="$3"

  if grep -Fq -- "$pattern" "$file"; then
    fail "${label}: unexpected '${pattern}'"
  fi
}

assert_equal "$(printf '\n' | prompt_yes_no "Start container on boot?")" "1" "empty onboot default"
assert_equal "$(printf 'y\n' | prompt_yes_no "Start container on boot?")" "1" "lowercase y onboot"
assert_equal "$(printf 'Y\n' | prompt_yes_no "Start container on boot?")" "1" "uppercase Y onboot"
assert_equal "$(printf 'yes\n' | prompt_yes_no "Start container on boot?")" "1" "lowercase yes onboot"
assert_equal "$(printf 'YES\n' | prompt_yes_no "Start container on boot?")" "1" "uppercase YES onboot"
assert_equal "$(printf 'n\n' | prompt_yes_no "Start container on boot?")" "0" "lowercase n onboot"
assert_equal "$(printf 'N\n' | prompt_yes_no "Start container on boot?")" "0" "uppercase N onboot"
assert_equal "$(printf 'no\n' | prompt_yes_no "Start container on boot?")" "0" "lowercase no onboot"
assert_equal "$(printf 'NO\n' | prompt_yes_no "Start container on boot?")" "0" "uppercase NO onboot"

invalid_output="$(mktemp)"
assert_equal "$(printf 'maybe\nno\n' | prompt_yes_no "Start container on boot?" 2>"$invalid_output")" "0" "invalid onboot retry result"
assert_file_contains "$invalid_output" "Please enter yes or no." "invalid onboot retry message"
rm -f "$invalid_output"

assert_equal "$ONBOOT" "1" "onboot default state"
ONBOOT="1"
assert_equal "$(onboot_label)" "yes" "onboot yes summary label"
ONBOOT="0"
assert_equal "$(onboot_label)" "no" "onboot no summary label"
ONBOOT="1"
assert_file_contains "${script_dir}/lannventory.sh" '--onboot "$ONBOOT"' "pct create onboot variable"
assert_file_not_contains "${script_dir}/lannventory.sh" "--onboot 1" "pct create no hardcoded onboot"

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

expected_url="https://github.com/godlev/LANnventory/releases/download/v0.1.0-beta.2/lannventory_0.1.0-beta.2_linux_amd64.deb"
assert_equal "$(package_url_for_arch amd64)" "$expected_url" "amd64 package URL"
assert_equal "$(package_url_for_arch x86_64)" "$expected_url" "x86_64 package URL"
assert_failure package_url_for_arch arm64

printf 'LANnventory Proxmox installer helper tests passed\n'
