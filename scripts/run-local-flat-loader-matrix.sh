#!/usr/bin/env bash
set -Eeuo pipefail

archive="${1:-/tmp/rbn-data/20260821.zip}"
shift || true

qs_repo="${QS_REPO:-$HOME/projects/quantastream}"
app_repo="${APP_REPO:-$HOME/projects/radiosport-data-lab}"
data_dir="${RBN_LOAD_DATA_DIR:-/tmp/radiosport-loader-data}"
mysql_port="${RBN_LOAD_MYSQL_PORT:-4020}"
grpc_port="${RBN_LOAD_GRPC_PORT:-4120}"
loader_port="${RBN_LOAD_HTTP_PORT:-8089}"
load_timeout="${RBN_LOAD_TIMEOUT:-5m}"
limit="${RBN_LOAD_LIMIT:-0}"
summary="${RBN_LOAD_SUMMARY:-/tmp/radiosport-flat-loader-matrix.tsv}"

configs=("$@")
if [ "${#configs[@]}" -eq 0 ]; then
  configs=("1:1000" "2:1000" "4:1000" "8:1000")
fi

server_session="radiosport-loader-qs-${mysql_port}"
loader_session="radiosport-loader-http-${loader_port}"
server_log="/tmp/${server_session}.log"
loader_log="/tmp/${loader_session}.log"

stop_stack() {
  stop_loader
  tmux kill-session -t "$server_session" 2>/dev/null || true
  for port in "$mysql_port" "$grpc_port" "$loader_port"; do
    pids=$(ss -ltnp 2>/dev/null | grep -E ":${port}\b" | sed -n 's/.*pid=\([0-9][0-9]*\).*/\1/p' | sort -u | tr '\n' ' ' || true)
    if [ -n "$pids" ]; then
      kill $pids 2>/dev/null || true
    fi
  done
}

stop_loader() {
  if tmux has-session -t "$loader_session" 2>/dev/null; then
    tmux send-keys -t "$loader_session" C-c 2>/dev/null || true
  fi
  for _ in $(seq 1 300); do
    if ! tmux has-session -t "$loader_session" 2>/dev/null; then
      return 0
    fi
    sleep 1
  done
  echo "loader did not exit after shutdown request; killing ${loader_session}" >&2
  tmux kill-session -t "$loader_session" 2>/dev/null || true
}

wait_mysql() {
  for _ in $(seq 1 90); do
    if mysql -h 127.0.0.1 -P "$mysql_port" -u root -D quanta -e "select 1" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  tail -80 "$server_log" >&2 || true
  return 1
}

wait_loader() {
  for _ in $(seq 1 60); do
    if curl -fsS "http://127.0.0.1:${loader_port}/healthz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  tail -80 "$loader_log" >&2 || true
  return 1
}

wait_loader_idle() {
  local queued
  for _ in $(seq 1 600); do
    queued=$(curl -fsS "http://127.0.0.1:${loader_port}/stats" 2>/dev/null | python3 -c 'import json,sys; print(json.load(sys.stdin).get("router", {}).get("total_queued", -1))' 2>/dev/null || echo -1)
    if [ "$queued" = "0" ]; then
      return 0
    fi
    sleep 1
  done
  echo "loader did not drain to idle" >&2
  curl -fsS "http://127.0.0.1:${loader_port}/stats" 2>/dev/null | python3 -m json.tool >&2 || true
  return 1
}

start_stack() {
  local workers="$1"
  stop_stack
  rm -rf "$data_dir"
  mkdir -p "$data_dir"
  : > "$server_log"
  : > "$loader_log"

  tmux new-session -d -s "$server_session" \
    "cd $qs_repo && env QUANTASTREAM_CONFIG_DIR=$app_repo/configuration QUANTASTREAM_DATA_DIR=$data_dir QUANTASTREAM_BIND=127.0.0.1 QUANTASTREAM_MYSQL_PORT=$mysql_port QUANTASTREAM_NATIVE_GRPC_BIND=127.0.0.1 QUANTASTREAM_NATIVE_GRPC_PORT=$grpc_port QUANTASTREAM_DATABASE=quanta QUANTASTREAM_AUTH_MODE=permissive ./startup-scripts/start-standard.sh >> $server_log 2>&1"
  wait_mysql

  tmux new-session -d -s "$loader_session" \
    "cd $qs_repo && go run ./cmd/quantastream-loader -listen 127.0.0.1:$loader_port -config-dir $app_repo/configuration -database quanta -connection-mode standard-native -native-grpc-addr 127.0.0.1:$grpc_port -workers $workers -channel-size 500000 -physical-build-routing -tables spots_flat >> $loader_log 2>&1"
  wait_loader
}

printf 'config\tloader_workers\tbatch_size\tlimit\temitted\taccepted\tvisible_rows\tseconds\taccepted_per_second\tvisible_rows_per_second\n' > "$summary"

for config in "${configs[@]}"; do
  IFS=: read -r workers batch_size <<< "$config"
  if [ -z "${workers:-}" ] || [ -z "${batch_size:-}" ]; then
    echo "invalid config ${config}; expected workers:batch_size" >&2
    exit 2
  fi

  echo "===== flat loader matrix ${config} ====="
  start_stack "$workers"

  limit_args=()
  if [ "$limit" != "0" ]; then
    limit_args=(-limit "$limit")
  fi

  start_ns=$(date +%s%N)
  load_log=$(mktemp)
  if ! (
    cd "$app_repo"
    go run ./cmd/rbn-archive-load \
      -target "http://127.0.0.1:${loader_port}/ingest/json" \
      -batch-size "$batch_size" \
      -workers "$workers" \
      -timeout "$load_timeout" \
      -spot-type rbn_spot_flat \
      -qrz-parents=false \
      -dense-spot-ids \
      "${limit_args[@]}" \
      "$archive"
  ) 2>&1 | tee "$load_log"; then
    rm -f "$load_log"
    exit 1
  fi
  wait_loader_idle
  stop_loader
  end_ns=$(date +%s%N)

  emitted=$(sed -n 's/.*emitted=\([0-9][0-9]*\).*/\1/p' "$load_log" | tail -1)
  accepted=$(sed -n 's/.*accepted=\([0-9][0-9]*\).*/\1/p' "$load_log" | tail -1)
  rm -f "$load_log"
  rows=$(mysql -N -B -h 127.0.0.1 -P "$mysql_port" -u root -D quanta -e "select count(*) from spots_flat;")
  seconds=$(awk -v start="$start_ns" -v end="$end_ns" 'BEGIN { printf "%.3f", (end - start) / 1000000000 }')
  accepted_rps=$(awk -v rows="$accepted" -v seconds="$seconds" 'BEGIN { if (seconds > 0) printf "%.2f", rows / seconds; else printf "0.00" }')
  visible_rps=$(awk -v rows="$rows" -v seconds="$seconds" 'BEGIN { if (seconds > 0) printf "%.2f", rows / seconds; else printf "0.00" }')
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$config" "$workers" "$batch_size" "$limit" "$emitted" "$accepted" "$rows" "$seconds" "$accepted_rps" "$visible_rps" | tee -a "$summary"
done

echo "summary=${summary}"
