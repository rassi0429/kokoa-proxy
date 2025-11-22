#!/usr/bin/env bash
set -euo pipefail

COMMAND="${1:-}"
ZONE="${KOKOA_ZONE:-example.com}"
DNS_RECORDS_FILE="${DNS_RECORDS_FILE:-/var/lib/kokoa/dns-records.txt}"

usage() {
  cat <<EOF
Usage: $0 [add|delete|list] [options]
  add --name <sub> --ip <addr>    Add A record for <sub>.<zone>.
  delete --name <sub>             Remove record.
  list                            Show current records.
Environment:
  KOKOA_ZONE                      Base DNS zone (default: ${ZONE})
  DNS_RECORDS_FILE                Plain file used as a stand-in DNS backend.
EOF
}

ensure_store() {
  mkdir -p "$(dirname "$DNS_RECORDS_FILE")"
  touch "$DNS_RECORDS_FILE"
}

add_record() {
  local name="" ip=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --name) name="$2"; shift 2;;
      --ip) ip="$2"; shift 2;;
      *) echo "unknown flag $1"; exit 1;;
    esac
  done
  [[ -n "$name" && -n "$ip" ]] || { echo "name and ip required"; exit 1; }
  ensure_store
  echo "${name}.${ZONE} ${ip}" >> "$DNS_RECORDS_FILE"
  echo "added ${name}.${ZONE} -> ${ip}"
}

delete_record() {
  local name=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --name) name="$2"; shift 2;;
      *) echo "unknown flag $1"; exit 1;;
    esac
  done
  [[ -n "$name" ]] || { echo "name required"; exit 1; }
  ensure_store
  tmp="$(mktemp)"
  grep -v "^${name}\.${ZONE} " "$DNS_RECORDS_FILE" > "$tmp" || true
  mv "$tmp" "$DNS_RECORDS_FILE"
  echo "removed ${name}.${ZONE}"
}

list_records() {
  ensure_store
  cat "$DNS_RECORDS_FILE"
}

case "$COMMAND" in
  add) shift; add_record "$@";;
  delete) shift; delete_record "$@";;
  list) list_records;;
  *) usage; exit 1;;
esac
