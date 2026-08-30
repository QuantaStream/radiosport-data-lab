#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

default_archives=(
  /tmp/rbn-data/20231022.zip
  /tmp/rbn-data/20231104.zip
  /tmp/rbn-data/20231105.zip
)

args=("$@")
has_archive=0
after_double_dash=0
for arg in "${args[@]}"; do
  if [ "$after_double_dash" = "1" ]; then
    has_archive=1
    break
  fi
  case "$arg" in
    --)
      after_double_dash=1
      ;;
    -*)
      ;;
    *)
      has_archive=1
      break
      ;;
  esac
done

if [ "$has_archive" = "0" ]; then
  set -- \
    "${args[@]}" \
    "${default_archives[@]}"
fi

export CONTEST_STATIONS="${CONTEST_STATIONS:-8P5A,KP2M}"
export CONTEST_LOG_URLS="${CONTEST_LOG_URLS:-https://cqww.com/publiclogs/2023ph/8p5a.log,https://cqww.com/publiclogs/2023ph/6y1v.log,https://cqww.com/publiclogs/2023ph/kp2m.log}"
export RBN_CACHE_DIR="${RBN_CACHE_DIR:-/tmp/rbn-cache-cqww-ssb-2026-planning}"
export RBN_WORKFLOW_DIR="${RBN_WORKFLOW_DIR:-/tmp/radiosport-cqww-ssb-2026-planning-$(date -u +%Y%m%dT%H%M%SZ)}"
export RBN_DAY_WORKERS="${RBN_DAY_WORKERS:-1}"
export RBN_SPOT_WORKERS="${RBN_SPOT_WORKERS:-1}"
export RBN_ARCHIVE_LOADER_IDLE_TIMEOUT="${RBN_ARCHIVE_LOADER_IDLE_TIMEOUT:-2m}"
export RBN_ARCHIVE_COMMIT_AFTER_FILE="${RBN_ARCHIVE_COMMIT_AFTER_FILE:-1}"
export RBN_PARENT_FLUSH_WAIT="${RBN_PARENT_FLUSH_WAIT:-2s}"
export RBN_PROFILE_BUILD="${RBN_PROFILE_BUILD:-0}"
export RBN_STATION_ACTIVITY_BUILD="${RBN_STATION_ACTIVITY_BUILD:-1}"

# Slashed callsigns are valid radio callsigns. Pass them explicitly through
# CONTEST_STATIONS and CONTEST_LOG_URLS; do not infer a URL or filesystem path
# from the callsign.
exec "$script_dir/run-ti8x-contest-workflow.sh" "$@"
