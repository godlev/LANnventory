#!/usr/bin/env bash
set -euo pipefail

image="${1:-lannventory:ci}"
run_id="${GITHUB_RUN_ID:-local}-$(date +%s)-$$"
volume="lannventory-smoke-${run_id}"
first_container="lannventory-smoke-first-${run_id}"
second_container="lannventory-smoke-second-${run_id}"
tmpdir="$(mktemp -d)"

cleanup() {
  docker rm -f "${first_container}" "${second_container}" >/dev/null 2>&1 || true
  docker volume rm -f "${volume}" >/dev/null 2>&1 || true
  rm -rf "${tmpdir}"
}
trap cleanup EXIT

fail() {
  echo "Smoke test failed: $*" >&2
  echo "--- first container logs ---" >&2
  docker logs "${first_container}" >&2 2>/dev/null || true
  echo "--- second container logs ---" >&2
  docker logs "${second_container}" >&2 2>/dev/null || true
  exit 1
}

mkdir -p "${tmpdir}/no-scan-bin"
cat >"${tmpdir}/no-scan-bin/arp-scan" <<'EOF'
#!/usr/bin/env sh
echo "ERROR: arp-scan was executed during safe smoke test" >&2
date > /data/WatchYourLAN/ARP_SCAN_EXECUTED
exit 93
EOF
chmod +x "${tmpdir}/no-scan-bin/arp-scan"

docker volume create "${volume}" >/dev/null

start_container() {
  local name="$1"

  docker run -d \
    --name "${name}" \
    --network none \
    -v "${volume}:/data/WatchYourLAN" \
    -v "${tmpdir}/no-scan-bin:/tmp/no-scan-bin:ro" \
    -e "PATH=/tmp/no-scan-bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin" \
    -e "IFACES=" \
    -e "ARP_STRS=" \
    -e "ARP_STRS_JOINED=" \
    -e "USE_DB=sqlite" \
    -e "INFLUX_ENABLE=false" \
    -e "PROMETHEUS_ENABLE=false" \
    -e "PG_CONNECT=postgres://wyl:phase23b-pg-secret@example.invalid/wyl" \
    -e "INFLUX_TOKEN=phase23b-influx-secret" \
    -e "SHOUTRRR_URL=generic://phase23b-shout-secret" \
    "${image}" >/dev/null
}

wait_for_http_health() {
  local name="$1"

  for _ in $(seq 1 90); do
    if [ "$(docker inspect -f '{{.State.Running}}' "${name}")" = "true" ] &&
       [ "$(docker exec "${name}" curl -fsS -o /dev/null -w '%{http_code}' "http://127.0.0.1:8840/api/health" || true)" = "200" ]; then
      return 0
    fi
    sleep 1
  done

  fail "${name} did not return HTTP 200 from /api/health"
}

wait_for_docker_healthcheck() {
  local name="$1"

  for _ in $(seq 1 90); do
    status="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "${name}")"
    if [ "${status}" = "healthy" ]; then
      return 0
    fi
    sleep 1
  done

  fail "${name} Docker healthcheck did not become healthy"
}

assert_runtime_files_exist() {
  local name="$1"

  docker exec "${name}" sh -c 'test -e /data/WatchYourLAN/config_v2.yaml && test -e /data/WatchYourLAN/scan.db' ||
    fail "${name} did not create expected SQLite/config files"
}

assert_no_arp_scan() {
  local name="$1"

  docker exec "${name}" sh -c 'test ! -e /data/WatchYourLAN/ARP_SCAN_EXECUTED' ||
    fail "${name} executed arp-scan"
}

assert_secret_redaction() {
  local name="$1"
  local config_response

  config_response="$(docker exec "${name}" curl -fsS "http://127.0.0.1:8840/api/config")"
  if printf '%s' "${config_response}" | grep -Fq 'phase23b-'; then
    fail "/api/config exposed fake smoke-test secret material"
  fi
}

assert_ui_response() {
  local name="$1"
  local response_file="${tmpdir}/index.html"

  docker exec "${name}" curl -fsS "http://127.0.0.1:8840/" >"${response_file}"
  grep -Fq 'LANnventory' "${response_file}" || fail "root page did not contain LANnventory UI"
}

assert_no_external_runtime_assets() {
  local name="$1"
  local bundle_dir="${tmpdir}/served-assets"

  mkdir -p "${bundle_dir}"
  docker exec "${name}" curl -fsS "http://127.0.0.1:8840/" >"${bundle_dir}/index.html"
  docker exec "${name}" curl -fsS "http://127.0.0.1:8840/fs/public/assets/index.js" >"${bundle_dir}/index.js"
  docker exec "${name}" curl -fsS "http://127.0.0.1:8840/fs/public/assets/index.css" >"${bundle_dir}/index.css"
  docker exec "${name}" curl -fsS "http://127.0.0.1:8840/fs/public/assets/themes/sand/bootstrap.min.css" >"${bundle_dir}/sand.css"

  if grep -R -E 'cdn\.jsdelivr\.net|unpkg\.com|cdnjs\.cloudflare\.com|fonts\.googleapis\.com|fonts\.gstatic\.com|@import[^{;]*https?://|url\(["'\'']?https?://' "${bundle_dir}"; then
    fail "served UI includes an automatic external runtime asset reference"
  fi
}

stop_cleanly() {
  local name="$1"

  docker stop --time 30 "${name}" >/dev/null || fail "${name} did not stop with SIGTERM"
  exit_code="$(docker inspect -f '{{.State.ExitCode}}' "${name}")"
  [ "${exit_code}" = "0" ] || fail "${name} exited with code ${exit_code}"
  if docker logs "${name}" 2>&1 | grep -Eiq 'panic|fatal error'; then
    fail "${name} logs contain panic/fatal error text"
  fi
}

start_container "${first_container}"
wait_for_http_health "${first_container}"
wait_for_docker_healthcheck "${first_container}"
assert_ui_response "${first_container}"
assert_secret_redaction "${first_container}"
assert_no_external_runtime_assets "${first_container}"
assert_runtime_files_exist "${first_container}"
assert_no_arp_scan "${first_container}"
first_config_hash="$(docker exec "${first_container}" sha256sum /data/WatchYourLAN/config_v2.yaml | awk '{print $1}')"
stop_cleanly "${first_container}"
docker rm "${first_container}" >/dev/null

start_container "${second_container}"
wait_for_http_health "${second_container}"
wait_for_docker_healthcheck "${second_container}"
assert_ui_response "${second_container}"
assert_secret_redaction "${second_container}"
assert_no_external_runtime_assets "${second_container}"
assert_runtime_files_exist "${second_container}"
assert_no_arp_scan "${second_container}"
second_config_hash="$(docker exec "${second_container}" sha256sum /data/WatchYourLAN/config_v2.yaml | awk '{print $1}')"
[ "${first_config_hash}" = "${second_config_hash}" ] || fail "config_v2.yaml did not persist across container recreation"
stop_cleanly "${second_container}"

echo "LANnventory container smoke test passed"
