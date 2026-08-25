#!/usr/bin/env bash
set -Eeuo pipefail

qs_repo="${QS_REPO:-$HOME/quantastream}"
app_repo="${APP_REPO:-$HOME/radiosport-data-lab}"
loader_url="${RBN_LOADER_URL:-http://127.0.0.1:8088}"
mysql_host="${RBN_MYSQL_HOST:-127.0.0.1}"
mysql_port="${RBN_MYSQL_PORT:-4000}"
mysql_user="${RBN_MYSQL_USER:-qstream}"
mysql_db="${RBN_MYSQL_DB:-quanta}"
batch_size="${RBN_BATCH_SIZE:-2000}"
file_workers="${RBN_FILE_WORKERS:-1}"
day_workers="${RBN_DAY_WORKERS:-12}"
timeout="${RBN_LOAD_TIMEOUT:-20m}"
drain_wait_seconds="${RBN_DRAIN_WAIT_SECONDS:-900}"
visible_wait_seconds="${RBN_VISIBLE_WAIT_SECONDS:-900}"
out_dir="${RBN_BENCHMARK_DIR:-/tmp/radiosport-aws-flat-benchmark-$(date -u +%Y%m%dT%H%M%SZ)}"

archives=("$@")
if [ "${#archives[@]}" -eq 0 ]; then
  archives=(
    /tmp/rbn-data/20260806.zip
    /tmp/rbn-data/20260807.zip
    /tmp/rbn-data/20260810.zip
    /tmp/rbn-data/20260811.zip
    /tmp/rbn-data/20260812.zip
    /tmp/rbn-data/20260813.zip
    /tmp/rbn-data/20260814.zip
    /tmp/rbn-data/20260817.zip
    /tmp/rbn-data/20260818.zip
    /tmp/rbn-data/20260819.zip
    /tmp/rbn-data/20260820.zip
    /tmp/rbn-data/20260821.zip
  )
fi

mkdir -p "$out_dir"

log() {
  printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*" | tee -a "$out_dir/command.log"
}

mysql_exec() {
  mysql -h "$mysql_host" -P "$mysql_port" -u "$mysql_user" -D "$mysql_db" "$@"
}

capture_json() {
  local url="$1"
  local path="$2"
  curl -fsS "$url" | python3 -m json.tool > "$path"
}

spots_flat_count() {
  mysql -N -B -h "$mysql_host" -P "$mysql_port" -u "$mysql_user" -D "$mysql_db" \
    -e 'select count(*) from spots_flat;' | tr -d '\r'
}

wait_loader_idle() {
  local stats queued open_sessions
  for _ in $(seq 1 "$drain_wait_seconds"); do
    stats=$(curl -fsS "$loader_url/stats" 2>/dev/null || true)
    queued=$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("router", {}).get("total_queued", -1))' <<<"$stats" 2>/dev/null || echo -1)
    open_sessions=$(python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("router", {}).get("open_session_count", -1))' <<<"$stats" 2>/dev/null || echo -1)
    if [ "$queued" = "0" ] && [ "$open_sessions" = "0" ]; then
      return 0
    fi
    sleep 1
  done
  echo "loader did not drain within ${drain_wait_seconds}s" >&2
  curl -fsS "$loader_url/stats" 2>/dev/null | python3 -m json.tool >&2 || true
  return 1
}

wait_visible_rows() {
  local expected="$1"
  local rows=""
  for _ in $(seq 1 "$visible_wait_seconds"); do
    rows=$(spots_flat_count || echo -1)
    if [ "$rows" = "$expected" ]; then
      printf '%s' "$rows"
      return 0
    fi
    sleep 1
  done
  echo "spots_flat visible row count did not reach accepted count within ${visible_wait_seconds}s: rows=${rows} expected=${expected}" >&2
  return 1
}

run_query() {
  local label="$1"
  local sql="$2"
  local output="$out_dir/query-${label}.txt"
  local start_ns end_ns elapsed

  log "query ${label} start"
  start_ns=$(date +%s%N)
  mysql_exec -e "$sql" > "$output"
  end_ns=$(date +%s%N)
  elapsed=$(awk -v start="$start_ns" -v end="$end_ns" 'BEGIN { printf "%.3f", (end - start) / 1000000000 }')
  printf '%s\t%s\n' "$label" "$elapsed" | tee -a "$out_dir/query-timings.tsv"
  log "query ${label} elapsed=${elapsed}s output=${output}"
}

for archive in "${archives[@]}"; do
  if [ ! -f "$archive" ]; then
    echo "archive not found: $archive" >&2
    exit 2
  fi
done

log "output_dir=$out_dir"
log "qs_repo=$qs_repo"
log "app_repo=$app_repo"
log "loader_url=$loader_url"
log "mysql=${mysql_host}:${mysql_port}/${mysql_db} user=${mysql_user}"
log "batch_size=$batch_size file_workers=$file_workers day_workers=$day_workers timeout=$timeout drain_wait_seconds=$drain_wait_seconds visible_wait_seconds=$visible_wait_seconds"
printf '%s\n' "${archives[@]}" > "$out_dir/archives.txt"

log "checking cluster status"
(
  cd "$qs_repo"
  go run ./quanta-admin status
) | tee "$out_dir/cluster-status-before.txt"

log "checking loader health"
curl -fsS "$loader_url/healthz" | tee "$out_dir/loader-health-before.json"
printf '\n' >> "$out_dir/loader-health-before.json"
capture_json "$loader_url/stats" "$out_dir/loader-stats-before.json"

log "truncating spots_flat"
(
  cd "$qs_repo"
  go run ./quanta-admin truncate spots_flat
) | tee "$out_dir/truncate.log"
mysql_exec -e 'select count(*) as spots_flat_before from spots_flat;' | tee "$out_dir/count-before.txt"

load_log="$out_dir/archive-load.log"
log "archive load start"
start_ns=$(date +%s%N)
(
  cd "$app_repo"
  go run ./cmd/rbn-archive-load \
    -target "$loader_url/ingest/json" \
    -batch-size "$batch_size" \
    -workers "$file_workers" \
    -day-workers "$day_workers" \
    -timeout "$timeout" \
    -spot-type rbn_spot_flat \
    -qrz-parents=false \
    -dense-spot-ids \
    "${archives[@]}"
) 2>&1 | tee "$load_log"
client_end_ns=$(date +%s%N)
client_elapsed=$(awk -v start="$start_ns" -v end="$client_end_ns" 'BEGIN { printf "%.3f", (end - start) / 1000000000 }')
log "archive client elapsed=${client_elapsed}s"

accepted=$(sed -n 's/.*accepted=\([0-9][0-9]*\).*/\1/p' "$load_log" | tail -1)
failed=$(sed -n 's/.*failed=\([0-9][0-9]*\).*/\1/p' "$load_log" | tail -1)
if [ -z "$accepted" ] || [ -z "$failed" ]; then
  echo "could not parse archive load accepted/failed counts from $load_log" >&2
  exit 1
fi
if [ "$failed" != "0" ]; then
  echo "archive load reported failed=$failed" >&2
  exit 1
fi

capture_json "$loader_url/stats" "$out_dir/loader-stats-after-client-return.json"
log "waiting for loader drain"
wait_loader_idle
log "waiting for visible rows accepted=${accepted}"
visible_rows=$(wait_visible_rows "$accepted")
visible_end_ns=$(date +%s%N)
visible_elapsed=$(awk -v start="$start_ns" -v end="$visible_end_ns" 'BEGIN { printf "%.3f", (end - start) / 1000000000 }')
log "visible rows reached accepted=${accepted} elapsed=${visible_elapsed}s"

capture_json "$loader_url/stats" "$out_dir/loader-stats-after-drain.json"
mysql_exec -e 'select count(*) as spots_flat_count from spots_flat;' | tee "$out_dir/count-after.txt"

client_rows_per_second=$(awk -v rows="$accepted" -v seconds="$client_elapsed" 'BEGIN { if (seconds > 0) printf "%.2f", rows / seconds; else printf "0.00" }')
visible_rows_per_second=$(awk -v rows="$visible_rows" -v seconds="$visible_elapsed" 'BEGIN { if (seconds > 0) printf "%.2f", rows / seconds; else printf "0.00" }')

{
  printf 'metric\tvalue\n'
  printf 'archives\t%s\n' "${#archives[@]}"
  printf 'accepted\t%s\n' "$accepted"
  printf 'failed\t%s\n' "$failed"
  printf 'visible_rows\t%s\n' "$visible_rows"
  printf 'client_elapsed_seconds\t%s\n' "$client_elapsed"
  printf 'visible_elapsed_seconds\t%s\n' "$visible_elapsed"
  printf 'accepted_client_rows_per_second\t%s\n' "$client_rows_per_second"
  printf 'visible_rows_per_second\t%s\n' "$visible_rows_per_second"
  printf 'batch_size\t%s\n' "$batch_size"
  printf 'file_workers\t%s\n' "$file_workers"
  printf 'day_workers\t%s\n' "$day_workers"
  printf 'drain_wait_seconds\t%s\n' "$drain_wait_seconds"
  printf 'visible_wait_seconds\t%s\n' "$visible_wait_seconds"
} | tee "$out_dir/summary.tsv"

: > "$out_dir/query-timings.tsv"
printf 'query\tseconds\n' > "$out_dir/query-timings.tsv"
run_query "count_all" "select count(*) as spots_flat_count from spots_flat;"
run_query "like_n7_count" "select count(*) as n7_count from spots_flat where dx_call like 'N7%';"
run_query "like_oe_dl7uzo_count" "select count(*) as oe_dl7uzo_count from spots_flat where dx_call like 'OE/DL7UZO%';"
run_query "group_dx_prefix_top25" "select dx_prefix, count(*) as spots from spots_flat group by dx_prefix order by count(*) desc, dx_prefix limit 25;"
run_query "group_band_mode_top20" "select band, mode, count(*) as spots from spots_flat group by band, mode order by spots desc limit 20;"
run_query "filtered_signal_20m" "select dx_prefix, avg(signal_db) as avg_signal, count(*) as spots from spots_flat where band = '20m' group by dx_prefix order by spots desc limit 20;"

capture_json "$loader_url/stats" "$out_dir/loader-stats-after-queries.json"
(
  cd "$qs_repo"
  git rev-parse HEAD
  git status --short
) > "$out_dir/quantastream-git.txt"
(
  cd "$app_repo"
  git rev-parse HEAD
  git status --short
) > "$out_dir/radiosport-git.txt"

tarball="${out_dir}.tar.gz"
tar -C "$(dirname "$out_dir")" -czf "$tarball" "$(basename "$out_dir")"
log "summary=$out_dir/summary.tsv"
log "query_timings=$out_dir/query-timings.tsv"
log "tarball=$tarball"
