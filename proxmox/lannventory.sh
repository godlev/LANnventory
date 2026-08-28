#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="LANnventory"
REPO_URL="https://github.com/godlev/LANnventory"
RELEASE_TAG="v0.1.0-beta.1"
VERSION="0.1.0-beta.1"
DEFAULT_HOSTNAME="lannventory"
DEFAULT_CORES="1"
DEFAULT_RAM="512"
DEFAULT_SWAP="512"
DEFAULT_DISK_GB="4"
DEFAULT_NET_IFACE="eth0"
DEFAULT_WEB_PORT="8840"

CTID=""
HOSTNAME="$DEFAULT_HOSTNAME"
ROOTFS_STORAGE=""
TEMPLATE_STORAGE=""
BRIDGE=""
CORES="$DEFAULT_CORES"
RAM="$DEFAULT_RAM"
SWAP="$DEFAULT_SWAP"
DISK_GB="$DEFAULT_DISK_GB"
IP_CONFIG="dhcp"
GW_CONFIG=""
TEMPLATE_FILE=""
CREATED_CT="no"
INSTALL_STAGE="preflight"

die() {
  printf 'ERROR: %s\n' "$*" >&2
  exit 1
}

info() {
  printf '\n==> %s\n' "$*"
}

run_in_ct() {
  pct exec "$CTID" -- bash -lc "$1"
}

show_failure_diagnostics() {
  local exit_code=$?
  if [[ "$CREATED_CT" == "yes" ]]; then
    printf '\n%s installation did not complete.\n' "$APP_NAME" >&2
    printf 'Failure stage: %s\n' "$INSTALL_STAGE" >&2
    printf 'Container ID: %s\n' "$CTID" >&2
    printf 'Inspect with: pct enter %s\n\n' "$CTID" >&2
    if pct status "$CTID" 2>/dev/null | grep -q "status: running"; then
      printf 'Service status:\n' >&2
      pct exec "$CTID" -- systemctl status lannventory --no-pager -l >&2 || true
      printf '\nRecent journal:\n' >&2
      pct exec "$CTID" -- journalctl -u lannventory --no-pager -n 100 >&2 || true
    fi
  fi
  exit "$exit_code"
}

trap show_failure_diagnostics ERR

require_root() {
  [[ "${EUID}" -eq 0 ]] || die "Run this installer as root on the Proxmox VE host."
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "Required command not found: $1"
}

require_proxmox_host() {
  require_root
  for cmd in pveversion pct pvesm pveam curl qm ip awk sed grep sort head tail cut seq; do
    require_command "$cmd"
  done
  pveversion >/dev/null 2>&1 || die "This does not appear to be a Proxmox VE host."
}

prompt_default() {
  local prompt="$1"
  local default="$2"
  local value
  read -r -p "$prompt [$default]: " value
  printf '%s' "${value:-$default}"
}

prompt_number() {
  local prompt="$1"
  local default="$2"
  local value
  while true; do
    value="$(prompt_default "$prompt" "$default")"
    [[ "$value" =~ ^[0-9]+$ ]] && {
      printf '%s' "$value"
      return
    }
    printf 'Please enter a whole number.\n' >&2
  done
}

prompt_positive_number() {
  local prompt="$1"
  local default="$2"
  local value
  while true; do
    value="$(prompt_number "$prompt" "$default")"
    if (( value > 0 )); then
      printf '%s' "$value"
      return
    fi
    printf 'Please enter a number greater than zero.\n' >&2
  done
}

prompt_vmid() {
  local default="$1"
  local value
  while true; do
    value="$(prompt_number "CT ID" "$default")"
    if (( value >= 100 && value <= 999999 )); then
      printf '%s' "$value"
      return
    fi
    printf 'Please enter a CT ID between 100 and 999999.\n' >&2
  done
}

confirm() {
  local prompt="$1"
  local answer
  read -r -p "$prompt [Y/n]: " answer
  [[ -z "$answer" || "$answer" =~ ^[Yy]$ ]]
}

read_list() {
  local command_text="$1"
  local -n out_ref="$2"
  mapfile -t out_ref < <(eval "$command_text" | sed '/^[[:space:]]*$/d')
}

choose_from_list() {
  local title="$1"
  local default="$2"
  shift 2
  local options=("$@")
  local value

  [[ "${#options[@]}" -gt 0 ]] || die "No choices available for $title."
  printf '\n%s:\n' "$title" >&2
  printf '  %s\n' "${options[@]}" >&2
  while true; do
    value="$(prompt_default "Select $title" "$default")"
    for option in "${options[@]}"; do
      if [[ "$value" == "$option" ]]; then
        printf '%s' "$value"
        return
      fi
    done
    printf 'Invalid selection. Choose one of the listed values.\n' >&2
  done
}

get_next_vmid() {
  local next_id
  if command -v pvesh >/dev/null 2>&1; then
    next_id="$(pvesh get /cluster/nextid 2>/dev/null || true)"
    if [[ "$next_id" =~ ^[0-9]+$ ]]; then
      printf '%s' "$next_id"
      return
    fi
  fi

  for candidate in $(seq 100 999999); do
    if ! pct status "$candidate" >/dev/null 2>&1 && ! qm status "$candidate" >/dev/null 2>&1; then
      printf '%s' "$candidate"
      return
    fi
  done
  die "Could not determine a free CT ID."
}

ensure_vmid_free() {
  local vmid="$1"
  if pct status "$vmid" >/dev/null 2>&1; then
    die "CT ID $vmid is already used by an LXC container."
  fi
  if qm status "$vmid" >/dev/null 2>&1; then
    die "VMID $vmid is already used by a QEMU VM."
  fi
}

detect_rootfs_storages() {
  read_list "pvesm status -content rootdir 2>/dev/null | awk 'NR > 1 {print \$1}' | sort -u" ROOTFS_STORAGES
  [[ "${#ROOTFS_STORAGES[@]}" -gt 0 ]] || die "No Proxmox storage with rootdir content is available."
}

detect_template_storages() {
  read_list "pvesm status -content vztmpl 2>/dev/null | awk 'NR > 1 {print \$1}' | sort -u" TEMPLATE_STORAGES
  [[ "${#TEMPLATE_STORAGES[@]}" -gt 0 ]] || die "No Proxmox storage with vztmpl content is available."
}

detect_bridges() {
  read_list "ip -o link show type bridge | awk -F': ' '{print \$2}' | sed 's/@.*//' | sort -u" BRIDGES
  if [[ "${#BRIDGES[@]}" -eq 0 && -f /etc/network/interfaces ]]; then
    read_list "awk '/^iface[[:space:]]+vmbr[0-9]+/ {print \$2}' /etc/network/interfaces | sort -u" BRIDGES
  fi
  [[ "${#BRIDGES[@]}" -gt 0 ]] || die "No Linux bridge was detected. Create or select a Proxmox bridge before running this installer."
}

validate_hostname() {
  local value="$1"
  [[ "$value" =~ ^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$ ]] || die "Invalid hostname '$value'."
}

validate_ipv4_cidr() {
  local value="$1"
  [[ "$value" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}$ ]] || return 1
  local prefix="${value##*/}"
  (( prefix >= 0 && prefix <= 32 ))
}

validate_ipv4() {
  local value="$1"
  [[ "$value" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]]
}

select_debian_template() {
  local available newest template_path
  INSTALL_STAGE="Debian template discovery"
  info "Updating Proxmox template index"
  pveam update

  available="$(pveam available --section system | awk '$2 ~ /^debian-13-standard_.*_amd64\.tar\.(zst|xz|gz)$/ {print $2}' | sort -V)"
  newest="$(printf '%s\n' "$available" | sed '/^[[:space:]]*$/d' | tail -n 1)"
  [[ -n "$newest" ]] || die "No Debian 13 amd64 standard LXC template was found. Run 'pveam update' and verify Debian 13 templates are available."

  template_path="${TEMPLATE_STORAGE}:vztmpl/${newest}"
  if ! pveam list "$TEMPLATE_STORAGE" | awk '{print $1}' | grep -Fxq "$template_path"; then
    info "Downloading Debian 13 template to $TEMPLATE_STORAGE"
    pveam download "$TEMPLATE_STORAGE" "$newest"
  else
    info "Reusing existing template $template_path"
  fi

  TEMPLATE_FILE="$template_path"
}

detect_arch_package_url() {
  local arch
  arch="$(dpkg --print-architecture 2>/dev/null || uname -m)"
  case "$arch" in
    amd64|x86_64)
      printf '%s/releases/download/%s/lannventory_%s_linux_amd64.deb' "$REPO_URL" "$RELEASE_TAG" "$VERSION"
      ;;
    *)
      die "Unsupported host architecture '$arch'. This installer currently supports amd64 only."
      ;;
  esac
}

configure_interactively() {
  local default_ctid default_rootfs default_template default_bridge ip_mode static_ip static_gw

  default_ctid="$(get_next_vmid)"
  detect_rootfs_storages
  detect_template_storages
  detect_bridges

  default_rootfs="${ROOTFS_STORAGES[0]}"
  default_template="${TEMPLATE_STORAGES[0]}"
  default_bridge="${BRIDGES[0]}"

  printf '\n%s Proxmox LXC installer\n' "$APP_NAME"
  printf 'Repository: %s\n' "$REPO_URL"
  printf 'Release: %s\n' "$RELEASE_TAG"
  printf 'Detected Proxmox: %s\n' "$(pveversion | head -n 1)"

  CTID="$(prompt_vmid "$default_ctid")"
  ensure_vmid_free "$CTID"
  HOSTNAME="$(prompt_default "Hostname" "$DEFAULT_HOSTNAME")"
  validate_hostname "$HOSTNAME"
  ROOTFS_STORAGE="$(choose_from_list "rootfs storage" "$default_rootfs" "${ROOTFS_STORAGES[@]}")"
  TEMPLATE_STORAGE="$(choose_from_list "template storage" "$default_template" "${TEMPLATE_STORAGES[@]}")"
  BRIDGE="$(choose_from_list "Linux bridge" "$default_bridge" "${BRIDGES[@]}")"
  CORES="$(prompt_positive_number "CPU cores" "$DEFAULT_CORES")"
  RAM="$(prompt_positive_number "RAM in MB" "$DEFAULT_RAM")"
  SWAP="$(prompt_positive_number "Swap in MB" "$DEFAULT_SWAP")"
  DISK_GB="$(prompt_positive_number "Disk size in GB" "$DEFAULT_DISK_GB")"

  while true; do
    ip_mode="$(prompt_default "IPv4 mode: dhcp or static" "dhcp")"
    case "$ip_mode" in
      dhcp|DHCP)
        IP_CONFIG="dhcp"
        GW_CONFIG=""
        break
        ;;
      static|STATIC)
        read -r -p "Static IPv4 address/CIDR: " static_ip
        read -r -p "Static IPv4 gateway: " static_gw
        if ! validate_ipv4_cidr "$static_ip" || ! validate_ipv4 "$static_gw"; then
          printf 'Static IPv4 requires address/CIDR like 192.168.1.50/24 and a gateway like 192.168.1.1.\n' >&2
          continue
        fi
        IP_CONFIG="$static_ip"
        GW_CONFIG="$static_gw"
        break
        ;;
      *)
        printf 'Please enter dhcp or static.\n' >&2
        ;;
    esac
  done

  printf '\nConfiguration summary:\n'
  printf '  CT ID: %s\n' "$CTID"
  printf '  Hostname: %s\n' "$HOSTNAME"
  printf '  Debian template storage: %s\n' "$TEMPLATE_STORAGE"
  printf '  Rootfs storage: %s\n' "$ROOTFS_STORAGE"
  printf '  Bridge: %s\n' "$BRIDGE"
  printf '  CPU: %s core(s)\n' "$CORES"
  printf '  RAM: %s MB\n' "$RAM"
  printf '  Swap: %s MB\n' "$SWAP"
  printf '  Disk: %s GB\n' "$DISK_GB"
  printf '  IPv4: %s\n' "$IP_CONFIG"
  printf '  Container type: unprivileged LXC\n'
  printf '  Start on boot: yes\n'
  confirm "Create this container?" || die "Installation cancelled before creating a container."
}

create_container() {
  local net_config
  INSTALL_STAGE="container creation"
  ensure_vmid_free "$CTID"
  select_debian_template

  net_config="name=${DEFAULT_NET_IFACE},bridge=${BRIDGE},ip=${IP_CONFIG}"
  if [[ -n "$GW_CONFIG" ]]; then
    net_config="${net_config},gw=${GW_CONFIG}"
  fi

  info "Creating unprivileged Debian 13 LXC $CTID"
  pct create "$CTID" "$TEMPLATE_FILE" \
    --hostname "$HOSTNAME" \
    --unprivileged 1 \
    --cores "$CORES" \
    --memory "$RAM" \
    --swap "$SWAP" \
    --rootfs "${ROOTFS_STORAGE}:${DISK_GB}" \
    --net0 "$net_config" \
    --onboot 1 \
    --ostype debian
  CREATED_CT="yes"
}

wait_for_container_network() {
  local waited ip
  INSTALL_STAGE="container network wait"
  info "Starting container $CTID"
  pct start "$CTID"

  for waited in $(seq 1 60); do
    if pct status "$CTID" 2>/dev/null | grep -q "status: running"; then
      break
    fi
    sleep 2
  done
  pct status "$CTID" | grep -q "status: running" || die "Container did not start within the timeout."

  for waited in $(seq 1 90); do
    if run_in_ct "ip link show ${DEFAULT_NET_IFACE} >/dev/null 2>&1"; then
      if [[ "$IP_CONFIG" == "dhcp" ]]; then
        ip="$(get_container_ip || true)"
        [[ -n "$ip" ]] && return
      else
        ip="$(get_container_ip || true)"
        [[ -n "$ip" ]] && return
      fi
    fi
    sleep 2
  done

  pct exec "$CTID" -- ip addr show >&2 || true
  die "Container network was not ready within the timeout."
}

get_container_ip() {
  pct exec "$CTID" -- ip -4 -o addr show dev "$DEFAULT_NET_IFACE" scope global 2>/dev/null \
    | awk '{print $4}' \
    | cut -d/ -f1 \
    | head -n 1
}

install_lannventory() {
  local package_url
  package_url="$(detect_arch_package_url)"

  INSTALL_STAGE="bootstrap packages"
  info "Installing bootstrap download requirements"
  run_in_ct "export DEBIAN_FRONTEND=noninteractive; apt-get update; apt-get install -y ca-certificates curl"

  INSTALL_STAGE="LANnventory package installation"
  info "Installing $APP_NAME $VERSION from official release package"
  run_in_ct "tmp_deb=\$(mktemp /tmp/lannventory.XXXXXX.deb); curl -fsSL '$package_url' -o \"\$tmp_deb\"; export DEBIAN_FRONTEND=noninteractive; apt-get install -y \"\$tmp_deb\"; rm -f \"\$tmp_deb\""

  INSTALL_STAGE="service validation"
  info "Enabling and starting lannventory.service"
  run_in_ct "systemctl daemon-reload; systemctl enable --now lannventory"
  run_in_ct "systemctl is-active --quiet lannventory"

  INSTALL_STAGE="health and version validation"
  info "Checking local health and version endpoints"
  run_in_ct "for i in \$(seq 1 30); do curl -fsS 'http://127.0.0.1:${DEFAULT_WEB_PORT}/api/health' >/dev/null && exit 0; sleep 2; done; exit 1"
  run_in_ct "version_response=\$(curl -fsS 'http://127.0.0.1:${DEFAULT_WEB_PORT}/api/version'); test \"\$version_response\" = '$VERSION' || test \"\$version_response\" = '\"$VERSION\"'"

  INSTALL_STAGE="arp-scan availability check"
  info "Checking arp-scan is installed without executing a scan"
  run_in_ct "command -v arp-scan >/dev/null"
}

print_success() {
  local ip url
  ip="$(get_container_ip || true)"
  if [[ -n "$ip" ]]; then
    url="http://${ip}:${DEFAULT_WEB_PORT}"
  else
    ip="not detected"
    url="http://<container-ip>:${DEFAULT_WEB_PORT}"
  fi

  cat <<SUMMARY

============================================================
LANnventory installed successfully
============================================================

Version:        ${VERSION}
Container ID:   ${CTID}
Hostname:       ${HOSTNAME}
Container type: Unprivileged LXC
IP:             ${ip}
URL:            ${url}
Service:        lannventory.service

Proxmox host commands:
  pct enter ${CTID}
  pct start ${CTID}
  pct stop ${CTID}

Inside container:
  systemctl status lannventory
  journalctl -u lannventory

============================================================

SUMMARY
}

main() {
  require_proxmox_host
  configure_interactively
  create_container
  wait_for_container_network
  install_lannventory
  trap - ERR
  print_success
}

main "$@"
