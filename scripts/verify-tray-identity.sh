#!/usr/bin/env bash
#
# R45 · read floter's tray identity back off the session bus and prove it is
# unique.
#
# The tray is a StatusNotifierItem. Every name a panel can route by (the `Id`
# property, the item object path, the dbusmenu path) is derived by
# libayatana-appindicator from one string, `tray-icon tray app <tray id>`. Two
# Tauri apps that pass the same tray id publish the same object path, so a host
# that keys items by that identity binds the wrong menu to the wrong icon.
#
# This script mirrors that derivation for the identifier in tauri.conf.json and
# then asserts, against the live bus, that
#
#   1. exactly one connection exposes an item whose `Id` is ours, and
#   2. floter is not among the connections using the shared example id.
#
# Run it with floter running, from the same session. Exits non-zero on a failed
# assertion.
#
#   scripts/verify-tray-identity.sh
#
# Set FLOTER_TRAY_PROCESS when the binary is not named `floter` (a dev build, a
# renamed install); it is only used to narrow the first scan.
#
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONFIG="${ROOT}/src-tauri/tauri.conf.json"
PROCESS="${FLOTER_TRAY_PROCESS:-floter}"

if ! command -v busctl >/dev/null 2>&1; then
  echo "skip: busctl not found (this check needs systemd's busctl)" >&2
  exit 0
fi
if ! command -v python3 >/dev/null 2>&1; then
  echo "skip: python3 not found" >&2
  exit 0
fi

IDENTIFIER="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["identifier"])' "$CONFIG")"
TRAY_ID="${IDENTIFIER}.tray"
APPINDICATOR_ID="tray-icon tray app ${TRAY_ID}"
clean() { printf '%s' "$1" | sed 's/[^A-Za-z0-9]/_/g'; }
OBJECT_PATH="/org/ayatana/NotificationItem/$(clean "$APPINDICATOR_ID")"
MENU_PATH="${OBJECT_PATH}/Menu"
LEGACY_ID="tray-icon tray app main-tray"
LEGACY_PATH="/org/ayatana/NotificationItem/$(clean "$LEGACY_ID")"

echo "floter tray identity (R45)"
echo "  tray id        : ${TRAY_ID}"
echo "  SNI Id         : ${APPINDICATOR_ID}"
echo "  SNI object path: ${OBJECT_PATH}"
echo "  dbusmenu path  : ${MENU_PATH}"
echo

# A connection whose owner has gone away can leave `busctl` blocked forever, so
# every read is bounded. `--user` is the session bus the tray lives on; run this
# from the same session as floter.
read_id() { # <bus name> <object path>
  local raw
  raw="$(timeout 3 busctl --user get-property "$1" "$2" \
        org.kde.StatusNotifierItem Id </dev/null 2>/dev/null || true)"
  raw="${raw#s }"
  printf '%s' "${raw//\"/}"
}

connections() { # <awk regexp matched against the PROCESS column>
  timeout 5 busctl --user list --no-pager --no-legend </dev/null 2>/dev/null \
    | awk -v who="$1" '$1 ~ /^:/ && $3 ~ who {print $1}'
}

candidates="$(connections "^${PROCESS}$")"
if [[ -z "$candidates" ]]; then
  echo "note: no '${PROCESS}' process is on this bus; scanning every connection"
  candidates="$(connections '.')"
fi

matches=0
legacy_in_floter=0
for name in $candidates; do
  id="$(read_id "$name" "$OBJECT_PATH")"
  if [[ "$id" == "$APPINDICATOR_ID" ]]; then
    matches=$((matches + 1))
    echo "  live item on ${name} -> Id='${id}'  (ours)"
  fi
  if [[ -n "$(read_id "$name" "$LEGACY_PATH")" ]]; then
    legacy_in_floter=$((legacy_in_floter + 1))
  fi
done

echo
fail=0
if [[ "$matches" -eq 1 ]]; then
  echo "PASS: exactly one connection registers Id '${APPINDICATOR_ID}'"
elif [[ "$matches" -eq 0 ]]; then
  echo "FAIL: no connection registers '${APPINDICATOR_ID}' — is floter running?"
  fail=1
else
  echo "FAIL: ${matches} connections register '${APPINDICATOR_ID}' — a duplicate re-creates the collision"
  fail=1
fi

if [[ "$legacy_in_floter" -gt 0 ]]; then
  echo "FAIL: a '${PROCESS}' connection still uses the shared example id '${LEGACY_ID}'"
  fail=1
else
  echo "PASS: no '${PROCESS}' connection uses the shared example id '${LEGACY_ID}'"
fi

# Informational: another Tauri app keeping the template default is the case this
# round stops colliding with.
others=0
for name in $(connections '.'); do
  if [[ -n "$(read_id "$name" "$LEGACY_PATH")" ]]; then
    others=$((others + 1))
  fi
done
if [[ "$others" -gt 0 ]]; then
  echo "NOTE: ${others} other connection(s) register '${LEGACY_ID}' — the collision"
  echo "      this round removes, and floter is no longer one of them."
fi

exit "$fail"
