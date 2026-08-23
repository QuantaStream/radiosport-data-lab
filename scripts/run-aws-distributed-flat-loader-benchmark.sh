#!/usr/bin/env bash
set -Eeuo pipefail

qs_repo="${QS_REPO:-$HOME/quantastream}"
app_repo="${APP_REPO:-$HOME/radiosport-data-lab}"
loader_url="${RBN_LOADER_URL:-http://127.0.0.1:8088}"
mysql_host="${RBN_MYSQL_HOST:-127.0.0.1}"
mysql_port="${RBN_MYSQL_PORT:-4000}"
mysql_user="${RBN_MYSQL_USER:-MOLIG004}"
mysql_db="${RBN_MYSQL_DB:-quanta}"
batch_size="${RBN_BATCH_SIZE:-2000}"
file_workers="${RBN_FILE_WORKERS:-1}"
day_workers="${RBN_DAY_WORKERS:-12}"
timeout="${RBN_LOAD_TIMEOUT:-20m}"
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
log "batch_size=$batch_size file_workers=$file_workers day_workers=$day_workers timeout=$timeout"
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
end_ns=$(date +%s%N)
elapsed=$(awk -v start="$start_ns" -v end="$end_ns" 'BEGIN { printf "%.3f", (end - start) / 1000000000 }')
log "archive load elapsed=${elapsed}s"

capture_json "$loader_url/stats" "$out_dir/loader-stats-after-load.json"
mysql_exec -e 'select count(*) as spots_flat_count from spots_flat;' | tee "$out_dir/count-after.txt"

accepted=$(sed -n 's/.*accepted=\([0-9][0-9]*\).*/\1/p' "$load_log" | tail -1)
failed=$(sed -n 's/.*failed=\([0-9][0-9]*\).*/\1/p' "$load_log" | tail -1)
visible_rows=$(mysql -N -B -h "$mysql_host" -P "$mysql_port" -u "$mysql_user" -D "$mysql_db" -e 'select count(*) from spots_flat;')
rows_per_second=$(awk -v rows="$accepted" -v seconds="$elapsed" 'BEGIN { if (seconds > 0) printf "%.2f", rows / seconds; else printf "0.00" }')

{
  printf 'metric\tvalue\n'
  printf 'archives\t%s\n' "${#archives[@]}"
  printf 'accepted\t%s\n' "$accepted"
  printf 'failed\t%s\n' "$failed"
  printf 'visible_rows\t%s\n' "$visible_rows"
  printf 'elapsed_seconds\t%s\n' "$elapsed"
  printf 'accepted_rows_per_second\t%s\n' "$rows_per_second"
  printf 'batch_size\t%s\n' "$batch_size"
  printf 'file_workers\t%s\n' "$file_workers"
  printf 'day_workers\t%s\n' "$day_workers"
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
