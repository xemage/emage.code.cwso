#!/usr/bin/env bash
# Host capability probe for containerized execution.
#
# Usage:
#   host_probe.sh
#   host_probe.sh --help
#
# Optional mount for host binary detection:
#   -v /:/host:ro
#
# Exit behavior:
# - 0 for successful probe output (even when capabilities are absent)
# - non-zero only for invalid usage

set -uo pipefail

usage() {
  cat <<'EOF'
Usage: host_probe.sh [--help]

Print host capability probe JSON with deterministic key ordering:
  kvm (bool)
  vhost_net (bool)
  kernel_version (string)
  kernel_gte_5_10 (bool)
  runsc_ok (bool)
  firecracker_ok (bool)

Run from a container context. Missing capabilities are reported as false.
EOF
}

if [[ $# -gt 1 ]]; then
  printf 'error: too many arguments\n' >&2
  usage >&2
  exit 64
fi

if [[ $# -eq 1 ]]; then
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'error: unknown argument: %s\n' "$1" >&2
      usage >&2
      exit 64
      ;;
  esac
fi

json_escape() {
  local value="$1"
  value=${value//\\/\\\\}
  value=${value//\"/\\\"}
  printf '%s' "$value"
}

device_rw() {
  local dev_path="$1"
  [[ -c "$dev_path" && -r "$dev_path" && -w "$dev_path" ]]
}

check_bin_version() {
  local bin_path="$1"
  "$bin_path" --version >/dev/null 2>&1
}

check_firecracker_bin() {
  local bin_path="$1"
  "$bin_path" --version >/dev/null 2>&1 || "$bin_path" --help >/dev/null 2>&1
}

kernel_version="$(uname -r 2>/dev/null || printf 'unknown')"
kernel_major=0
kernel_minor=0

if [[ "$kernel_version" =~ ^([0-9]+)\.([0-9]+) ]]; then
  kernel_major="${BASH_REMATCH[1]}"
  kernel_minor="${BASH_REMATCH[2]}"
fi

kernel_gte_5_10=false
if (( kernel_major > 5 )); then
  kernel_gte_5_10=true
elif (( kernel_major == 5 && kernel_minor >= 10 )); then
  kernel_gte_5_10=true
fi

kvm=false
if device_rw "/dev/kvm"; then
  kvm=true
fi

vhost_net=false
if device_rw "/dev/vhost-net" || device_rw "/dev/vhost_net"; then
  vhost_net=true
fi

runsc_ok=false
runsc_candidates=()
if command -v runsc >/dev/null 2>&1; then
  runsc_candidates+=("$(command -v runsc)")
fi
if [[ -x "/host/usr/local/bin/runsc" ]]; then
  runsc_candidates+=("/host/usr/local/bin/runsc")
fi
if [[ -x "/host/usr/bin/runsc" ]]; then
  runsc_candidates+=("/host/usr/bin/runsc")
fi
for candidate in "${runsc_candidates[@]}"; do
  if check_bin_version "$candidate"; then
    runsc_ok=true
    break
  fi
done

firecracker_bin_ok=false
firecracker_candidates=()
if command -v firecracker >/dev/null 2>&1; then
  firecracker_candidates+=("$(command -v firecracker)")
fi
if [[ -x "/host/usr/local/bin/firecracker" ]]; then
  firecracker_candidates+=("/host/usr/local/bin/firecracker")
fi
if [[ -x "/host/usr/bin/firecracker" ]]; then
  firecracker_candidates+=("/host/usr/bin/firecracker")
fi
for candidate in "${firecracker_candidates[@]}"; do
  if check_firecracker_bin "$candidate"; then
    firecracker_bin_ok=true
    break
  fi
done

firecracker_ok=false
if [[ "$kvm" == "true" && "$vhost_net" == "true" && "$kernel_gte_5_10" == "true" && "$firecracker_bin_ok" == "true" ]]; then
  firecracker_ok=true
fi

printf '{\n'
printf '  "kvm": %s,\n' "$kvm"
printf '  "vhost_net": %s,\n' "$vhost_net"
printf '  "kernel_version": "%s",\n' "$(json_escape "$kernel_version")"
printf '  "kernel_gte_5_10": %s,\n' "$kernel_gte_5_10"
printf '  "runsc_ok": %s,\n' "$runsc_ok"
printf '  "firecracker_ok": %s\n' "$firecracker_ok"
printf '}\n'
