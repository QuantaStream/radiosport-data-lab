#!/usr/bin/env bash
set -Eeuo pipefail

app_repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
qs_repo="${QS_REPO:-$HOME/projects/quantastream}"
loader_url="${RBN_LOADER_URL:-http://127.0.0.1:8088}"
mysql_host="${RBN_MYSQL_HOST:-127.0.0.1}"
mysql_port="${RBN_MYSQL_PORT:-4000}"
mysql_user="${RBN_MYSQL_USER:-qstream}"
mysql_db="${RBN_MYSQL_DB:-quanta}"
primary_station="${CONTEST_STATION:-TI8X}"
contest_logs_csv="${CONTEST_LOG_URLS:-${CONTEST_LOG_URL:-https://cqww.com/publiclogs/2025cw/ti8x.log}}"
stations_csv="${CONTEST_STATIONS:-$primary_station}"
cache_dir="${RBN_CACHE_DIR:-/tmp/rbn-cache-ti8x}"
cty_dat="${RBN_CTY_DAT:-$app_repo/data/cty/cty.dat}"
batch_size="${RBN_BATCH_SIZE:-1000}"
spot_batch_size="${RBN_SPOT_BATCH_SIZE:-2000}"
spot_workers="${RBN_SPOT_WORKERS:-4}"
day_workers="${RBN_DAY_WORKERS:-2}"
window="${CONTEST_MATCH_WINDOW:-5m}"
frequency_tolerance="${CONTEST_FREQUENCY_TOLERANCE_KHZ:-0}"
max_matches="${CONTEST_MAX_MATCHES_PER_QSO:-0}"
parent_flush_wait="${RBN_PARENT_FLUSH_WAIT:-2s}"
loader_wait_seconds="${RBN_LOADER_WAIT_SECONDS:-900}"
profile_build="${RBN_PROFILE_BUILD:-1}"
station_activity_build="${RBN_STATION_ACTIVITY_BUILD:-1}"
station_activity_dx_call="${RBN_STATION_ACTIVITY_DX_CALL:-}"
out_dir="${RBN_WORKFLOW_DIR:-/tmp/radiosport-ti8x-workflow-$(date -u +%Y%m%dT%H%M%SZ)}"
reset=0
install_views=1
create_schema=1

usage() {
  cat <<USAGE
usage: run-ti8x-contest-workflow.sh [--reset] [--skip-schema] [--skip-views] [archive.zip ...]

Runs the focused TI8X CQ WW CW 2025 workflow against an already running
QuantaStream server and qstream-loader:

  1. create the required QS tables when needed
  2. optionally truncate workflow tables child-first with --reset
  3. load SWPC propagation rows
  4. load focused RBN spots_flat rows for one DX call or a small competitor pack
  5. build a parsed RBN day/callsign cache
  6. load one or more Cabrillo logs
  7. materialize exact spot/QSO matches from the cache for each log
  8. build spotter calibration profiles
  9. install analyst views and print verification queries

Environment overrides:
  QS_REPO=$qs_repo
  RBN_LOADER_URL=$loader_url
  RBN_MYSQL_HOST=$mysql_host
  RBN_MYSQL_PORT=$mysql_port
  RBN_MYSQL_USER=$mysql_user
  RBN_MYSQL_DB=$mysql_db
  CONTEST_STATION=$primary_station
  CONTEST_STATIONS=$stations_csv
  CONTEST_LOG_URLS=$contest_logs_csv
  RBN_CACHE_DIR=$cache_dir
  RBN_STATION_ACTIVITY_BUILD=$station_activity_build
  RBN_STATION_ACTIVITY_DX_CALL=$station_activity_dx_call
  RBN_PROFILE_BUILD=$profile_build
USAGE
}

archives=()
while [ "$#" -gt 0 ]; do
  case "$1" in
    --reset)
      reset=1
      shift
      ;;
    --skip-schema)
      create_schema=0
      shift
      ;;
    --skip-views)
      install_views=0
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      while [ "$#" -gt 0 ]; do
        archives+=("$1")
        shift
      done
      ;;
    -*)
      echo "unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
    *)
      archives+=("$1")
      shift
      ;;
  esac
done

split_csv() {
  local input="$1"
  tr ',' '\n' <<<"$input" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' | sed '/^$/d'
}

join_csv() {
  local IFS=,
  printf '%s' "$*"
}

sql_string_list() {
  local value escaped sep=""
  for value in "$@"; do
    escaped="$(printf '%s' "$value" | sed "s/'/''/g")"
    printf "%s'%s'" "$sep" "$escaped"
    sep=","
  done
}

shell_quote() {
  printf '%q' "$1"
}

shell_args() {
  local value
  for value in "$@"; do
    printf ' %q' "$value"
  done
}

safe_label() {
  printf '%s' "$1" | tr -c 'A-Za-z0-9_.-' '_'
}

mapfile -t stations < <(split_csv "$stations_csv")
if [ "${#stations[@]}" -eq 0 ]; then
  echo "CONTEST_STATIONS or CONTEST_STATION must name at least one DX callsign" >&2
  exit 2
fi
station_filter="$(join_csv "${stations[@]}")"
station_sql_list="$(sql_string_list "${stations[@]}")"

if [ -n "$station_activity_dx_call" ]; then
  mapfile -t station_activity_calls < <(split_csv "$station_activity_dx_call")
else
  station_activity_calls=("${stations[@]}")
fi

mapfile -t contest_logs < <(split_csv "$contest_logs_csv")
if [ "${#contest_logs[@]}" -eq 0 ]; then
  echo "CONTEST_LOG_URLS or CONTEST_LOG_URL must name at least one Cabrillo log source" >&2
  exit 2
fi

if [ "${#archives[@]}" -eq 0 ]; then
  archives=(
    /tmp/rbn-data/20251129.zip
    /tmp/rbn-data/20251130.zip
  )
fi
mapfile -t archives < <(printf '%s\n' "${archives[@]}" | sort)

mkdir -p "$out_dir"

log() {
  printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" | tee -a "$out_dir/workflow.log"
}

mysql_exec() {
  mysql -h "$mysql_host" -P "$mysql_port" -u "$mysql_user" -D "$mysql_db" "$@"
}

admin() {
  (cd "$qs_repo" && go run ./qstream-admin "$@")
}

capture_loader_stats() {
  local label="$1"
  curl -fsS "$loader_url/stats" | python3 -m json.tool > "$out_dir/loader-stats-${label}.json"
}

wait_loader_idle() {
  local stats queued open_sessions
  for _ in $(seq 1 "$loader_wait_seconds"); do
    stats=$(curl -fsS "$loader_url/stats" 2>/dev/null || true)
    queued=$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("router", {}).get("total_queued", -1))' <<<"$stats" 2>/dev/null || echo -1)
    open_sessions=$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("router", {}).get("open_session_count", -1))' <<<"$stats" 2>/dev/null || echo -1)
    if [ "$queued" = "0" ] && [ "$open_sessions" = "0" ]; then
      return 0
    fi
    sleep 1
  done
  echo "loader did not drain within ${loader_wait_seconds}s" >&2
  curl -fsS "$loader_url/stats" 2>/dev/null | python3 -m json.tool >&2 || true
  return 1
}

archive_day() {
  local base day
  base="$(basename "$1")"
  day="${base:0:8}"
  if [[ ! "$day" =~ ^[0-9]{8}$ ]]; then
    echo "archive path does not start with YYYYMMDD: $1" >&2
    return 1
  fi
  printf '%s-%s-%s\n' "${day:0:4}" "${day:4:2}" "${day:6:2}"
}

run_step() {
  local label="$1"
  shift
  log "${label} start"
  "$@" 2>&1 | tee "$out_dir/${label}.log"
  log "${label} complete"
}

run_mysql_file() {
  local path="$1"
  log "install view $(basename "$path")"
  mysql_exec < "$path" 2>&1 | tee -a "$out_dir/views.log"
}

for archive in "${archives[@]}"; do
  if [ ! -f "$archive" ]; then
    echo "archive not found: $archive" >&2
    exit 2
  fi
done

cache_days=()
for archive in "${archives[@]}"; do
  cache_days+=("$(archive_day "$archive")")
done
from_day="${cache_days[0]}"
to_day="${cache_days[$((${#cache_days[@]} - 1))]}"
swpc_year="${from_day:0:4}"

cty_args=()
if [ -f "$cty_dat" ]; then
  cty_args=(-cty-dat "$cty_dat")
else
  log "warning: CTY file not found at $cty_dat; enrichment will use command defaults"
fi

app_repo_q="$(shell_quote "$app_repo")"
loader_ingest_q="$(shell_quote "$loader_url/ingest/json")"
cache_dir_q="$(shell_quote "$cache_dir")"
station_filter_q="$(shell_quote "$station_filter")"
from_day_q="$(shell_quote "$from_day")"
to_day_end_q="$(shell_quote "$to_day 23:59:59")"
parent_flush_wait_q="$(shell_quote "$parent_flush_wait")"
mysql_dsn_q="$(shell_quote "${mysql_user}@tcp(${mysql_host}:${mysql_port})/${mysql_db}?parseTime=true")"
archives_args="$(shell_args "${archives[@]}")"
cache_days_args="$(shell_args "${cache_days[@]}")"
contest_logs_args="$(shell_args "${contest_logs[@]}")"
cty_args_string="$(shell_args "${cty_args[@]}")"

log "output_dir=$out_dir"
log "qs_repo=$qs_repo"
log "app_repo=$app_repo"
log "loader_url=$loader_url"
log "mysql=${mysql_host}:${mysql_port}/${mysql_db} user=${mysql_user}"
log "dx_calls=$station_filter contest_logs=${contest_logs[*]}"
log "cache_dir=$cache_dir"
log "station_activity_build=$station_activity_build station_activity_dx_calls=${station_activity_calls[*]}"
printf '%s\n' "${archives[@]}" > "$out_dir/archives.txt"
printf '%s\n' "${cache_days[@]}" > "$out_dir/cache-days.txt"
printf '%s\n' "${stations[@]}" > "$out_dir/stations.txt"
printf '%s\n' "${contest_logs[@]}" > "$out_dir/contest-logs.txt"

log "checking MySQL endpoint"
mysql_exec -e 'select 1;' > "$out_dir/mysql-check.txt"
log "checking loader health"
curl -fsS "$loader_url/healthz" | tee "$out_dir/loader-health.json"
printf '\n' >> "$out_dir/loader-health.json"
capture_loader_stats "before"

if [ "$create_schema" = "1" ]; then
  for table in swpc_daily_indices swpc_k_indices_3h activity_5m_buckets spots_flat contest_logs contest_qsos contest_spot_matches rbn_spotter_nodes spotter_profile_snapshots spotter_profiles station_activity_5m_summaries; do
    log "create table ${table}"
    admin create --port="$mysql_port" --schema-dir="$app_repo/configuration" "$table" 2>&1 | tee -a "$out_dir/schema-create.log"
  done
fi

if [ "$reset" = "1" ]; then
  for table in station_activity_5m_summaries spotter_profile_snapshots spotter_profiles rbn_spotter_nodes contest_spot_matches contest_qsos spots_flat swpc_k_indices_3h contest_logs activity_5m_buckets swpc_daily_indices; do
    log "truncate table ${table}"
    admin truncate --port="$mysql_port" "$table" 2>&1 | tee -a "$out_dir/truncate.log"
  done
fi

run_step swpc-load \
  bash -lc "cd '$app_repo' && go run ./cmd/swpc-load -year '$swpc_year' -cache-dir data/swpc -from '$from_day' -to '$to_day' -target '$loader_url/ingest/json' -batch-size 100 -parent-flush-wait '$parent_flush_wait'"
wait_loader_idle
capture_loader_stats "after-swpc"

run_step rbn-archive-load \
  bash -lc "cd $app_repo_q && go run ./cmd/rbn-archive-load -target $loader_ingest_q -batch-size '$spot_batch_size' -workers '$spot_workers' -day-workers '$day_workers' -spot-type rbn_spot_flat -qrz-parents=false -dense-spot-ids -dx-call $station_filter_q$archives_args"
wait_loader_idle
capture_loader_stats "after-spots"

run_step rbn-cache-build \
  bash -lc "cd $app_repo_q && go run ./cmd/rbn-cache-build -cache-dir $cache_dir_q -dx-call $station_filter_q -dense-spot-ids$archives_args"

if [ "$station_activity_build" = "1" ]; then
  for activity_call in "${station_activity_calls[@]}"; do
    activity_call_q="$(shell_quote "$activity_call")"
    activity_label="$(safe_label "$activity_call")"
    run_step "station-activity-build-${activity_label}" \
      bash -lc "cd $app_repo_q && go run ./cmd/station-activity-build -mysql-dsn $mysql_dsn_q -target $loader_ingest_q -from $from_day_q -to $to_day_end_q -source-table spots_flat -batch-size '$batch_size' -dx-call $activity_call_q"
    wait_loader_idle
    capture_loader_stats "after-station-activity-${activity_label}"
  done
fi

if [ "$profile_build" = "1" ]; then
  run_step spotter-profile-build \
    bash -lc "cd $app_repo_q && go run ./cmd/spotter-profile-build -mysql-dsn $mysql_dsn_q -target $loader_ingest_q -from $from_day_q -to $to_day_end_q -profile-kind contest -source-table spots_flat -batch-size '$batch_size' -parent-flush-wait $parent_flush_wait_q$cty_args_string"
  wait_loader_idle
  capture_loader_stats "after-spotter-profiles"
fi

run_step cabrillo-load \
  bash -lc "cd $app_repo_q && go run ./cmd/cabrillo-load -target $loader_ingest_q -batch-size '$batch_size' -parent-flush-wait $parent_flush_wait_q$cty_args_string$contest_logs_args"
wait_loader_idle
capture_loader_stats "after-cabrillo"

for contest_log in "${contest_logs[@]}"; do
  contest_log_q="$(shell_quote "$contest_log")"
  contest_label="$(safe_label "$(basename "$contest_log")")"
  run_step "contest-spot-match-load-${contest_label}" \
    bash -lc "cd $app_repo_q && go run ./cmd/contest-spot-match-load -target $loader_ingest_q -batch-size '$batch_size'$cty_args_string -rbn-cache $cache_dir_q -window '$window' -frequency-tolerance-khz '$frequency_tolerance' -max-matches-per-qso '$max_matches' $contest_log_q$cache_days_args"
  wait_loader_idle
  capture_loader_stats "after-matches-${contest_label}"
done

if [ "$install_views" = "1" ]; then
  : > "$out_dir/views.log"
  run_mysql_file "$app_repo/sql/views/rbn_spot_propagation_base.sql"
  run_mysql_file "$app_repo/sql/views/contest_qso_propagation_base.sql"
  run_mysql_file "$app_repo/sql/views/contest_rbn_activity_5m_base.sql"
  run_mysql_file "$app_repo/sql/views/contest_spot_match_base.sql"
  run_mysql_file "$app_repo/sql/views/contest_best_spot_match_base.sql"
  run_mysql_file "$app_repo/sql/views/spotter_profile_base.sql"
  run_mysql_file "$app_repo/sql/views/station_activity_5m_base.sql"
fi

log "verification counts"
mysql_exec -e "
select count(*) as swpc_daily from swpc_daily_indices;
select count(*) as swpc_k_3h from swpc_k_indices_3h;
select count(*) as activity_buckets from activity_5m_buckets;
select count(*) as spots_flat_count, min(spotted_at), max(spotted_at) from spots_flat;
select count(*) as contest_log_count from contest_logs;
select count(*) as contest_qso_count, min(qso_at), max(qso_at) from contest_qsos;
select count(*) as contest_spot_match_count from contest_spot_matches;
select count(*) as spotter_profile_count from spotter_profiles;
select count(*) as station_activity_5m_count from station_activity_5m_summaries;
" | tee "$out_dir/counts.txt"

log "verification match summary"
mysql_exec -e "
select station_call, band, match_kind, count(*) as matches
from contest_spot_match_base
where station_call in ($station_sql_list)
group by station_call, band, match_kind
order by station_call, matches desc
limit 60;
" | tee "$out_dir/match-summary.txt"

log "verification best-match summary"
mysql_exec -e "
select station_call, band, count(*) as best_matches, avg(match_score) as avg_match_score
from contest_best_spot_match_base
where station_call in ($station_sql_list)
group by station_call, band
order by station_call, best_matches desc
limit 60;
" | tee "$out_dir/best-match-summary.txt"

log "verification spotter profile summary"
mysql_exec -e "
select spotter_call, spotter_continent, country_name, latitude, longitude, geo_confidence, total_spots, active_hours, avg_signal_db, spotter_weight, profile_quality
from spotter_profile_base
order by total_spots desc
limit 20;
" | tee "$out_dir/spotter-profile-summary.txt"

log "verification station activity summary"
mysql_exec -e "
select dx_call, band, spotter_continent, spot_count, distinct_spotters, avg_signal_db, reach_score
from station_activity_5m_base
where dx_call in ($station_sql_list)
order by dx_call, reach_score desc
limit 80;
" | tee "$out_dir/station-activity-summary.txt"

log "verification propagation summary"
mysql_exec -e "
select dx_call, band, dx_prefix, sfi, kp_index, count(*) as spots
from rbn_spot_propagation_base
where spotted_at between todate('$from_day') and todate('$to_day 23:59:59')
  and dx_call in ($station_sql_list)
group by dx_call, band, dx_prefix, sfi, kp_index
order by dx_call, spots desc
limit 80;
" | tee "$out_dir/propagation-summary.txt"

capture_loader_stats "final"
log "workflow complete"
log "artifacts=$out_dir"
