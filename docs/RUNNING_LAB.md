# Running The Local RadioSport Lab

This note is the durable restart/runbook for the local RadioSport Data Lab
environment. It assumes the QuantaStream and RadioSport repositories are checked
out side by side under `~/projects`.

## Paths

| Purpose | Path |
| --- | --- |
| QuantaStream repo | `~/projects/quantastream` |
| RadioSport repo | `~/projects/radiosport-data-lab` |
| RadioSport schema directory | `~/projects/radiosport-data-lab/configuration` |
| Local data directory | `~/qstream-data/rbn-cqww2025-standard/data` |
| RBN archive staging | `/tmp/rbn-data` |
| TI8X parsed RBN cache | `/tmp/rbn-cache-ti8x` |

## Ports

| Service | Address |
| --- | --- |
| QuantaStream MySQL-compatible endpoint | `127.0.0.1:4000` |
| QuantaStream native gRPC endpoint | `127.0.0.1:4100` |
| JSON loader HTTP endpoint | `127.0.0.1:8088` |

## Stop Existing Lab Processes

Use this before starting a release-candidate smoke, switching datasets, or
reloading from scratch:

```bash
tmux kill-session -t qs-radiosport 2>/dev/null || true
tmux kill-session -t qs-radiosport-loader 2>/dev/null || true

pkill -f '/tmp/qs-radiosport-quantastream' 2>/dev/null || true
pkill -f '/tmp/qs-radiosport-loader' 2>/dev/null || true
pkill -f './startup-scripts/start-standard.sh' 2>/dev/null || true
pkill -f 'cmd/quantastream' 2>/dev/null || true
pkill -f 'cmd/quantastream-loader' 2>/dev/null || true
```

Confirm the normal lab ports are quiet:

```bash
ss -ltnp | grep -E ':(4000|4100|8088)\b' || true
```

## Start QuantaStream

This starts the current source checkout in standard single-node mode, with
native gRPC enabled for the loader. `tmux` keeps the service alive after the
launching shell exits.

```bash
cd ~/projects/quantastream

go build -o /tmp/qs-radiosport-quantastream ./cmd/quantastream

tmux kill-session -t qs-radiosport 2>/dev/null || true
rm -f /tmp/qs-radiosport-standard.log

tmux new-session -d -s qs-radiosport "
exec env QUANTASTREAM_MYSQL_COMMAND_TRACE=true \
  /tmp/qs-radiosport-quantastream \
    -config-dir $HOME/projects/radiosport-data-lab/configuration \
    -data-dir $HOME/qstream-data/rbn-cqww2025-standard/data \
    -wal-path $HOME/qstream-data/rbn-cqww2025-standard/data/storage.wal \
    -bind 127.0.0.1 \
    -mysql-port 4000 \
    -native-grpc-bind 127.0.0.1 \
    -native-grpc-port 4100 \
    -database quanta \
    -auth-mode permissive \
    -runtime-probes=false \
  > /tmp/qs-radiosport-standard.log 2>&1"
```

Wait for the MySQL endpoint:

```bash
until mysqladmin ping -h 127.0.0.1 -P 4000 -u qstream --silent; do
  sleep 1
done
```

Useful log tail:

```bash
tail -f /tmp/qs-radiosport-standard.log
```

Attach to the owning terminal if needed:

```bash
tmux attach -t qs-radiosport
```

## Start The JSON Loader

Start the loader with every RadioSport table in the allowlist. Keeping the full
allowlist active makes it possible to load SWPC rows, RBN spots, Cabrillo QSOs,
materialized match rows, spotter profile rows, and station activity summaries
without restarting the loader.

```bash
cd ~/projects/quantastream

go build -o /tmp/qs-radiosport-loader ./cmd/quantastream-loader

tmux kill-session -t qs-radiosport-loader 2>/dev/null || true
rm -f /tmp/qs-radiosport-loader.log

tmux new-session -d -s qs-radiosport-loader "
exec /tmp/qs-radiosport-loader \
  -connection-mode standard-native \
  -native-grpc-addr 127.0.0.1:4100 \
  -listen 127.0.0.1:8088 \
  -config-dir $HOME/projects/radiosport-data-lab/configuration \
  -database quanta \
  -tables spots,spots_flat,qrz_callsigns,swpc_daily_indices,swpc_k_indices_3h,activity_5m_buckets,contest_logs,contest_qsos,contest_spot_matches,rbn_spotter_nodes,spotter_profile_snapshots,spotter_profiles,station_activity_5m_summaries \
  -workers 4 \
  -channel-size 100000 \
  -physical-build-routing \
  > /tmp/qs-radiosport-loader.log 2>&1"
```

Health and stats:

```bash
curl -fsS http://127.0.0.1:8088/healthz
curl -fsS http://127.0.0.1:8088/stats | python3 -m json.tool | head -120
```

For laptop loading, use the loader stats, table counts, and OS process RSS as
the operational signals. The QS admin status memory column is a best-effort
serialized cache-size estimate from the engine. It can skip busy shards and is
not a reliable capacity-planning number during active ingest.

Useful log tail:

```bash
tail -f /tmp/qs-radiosport-loader.log
```

## Quick Smoke

Connect with the MySQL CLI:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta
```

Or run the basic smoke directly:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta -e "
show tables;
select count(*) as spots_flat_count from spots_flat;
select count(*) as qso_count from contest_qsos;
select count(*) as match_count from contest_spot_matches;
select count(*) as spotter_profile_count from spotter_profiles;
select count(*) as station_activity_count from station_activity_5m_summaries;
"
```

The focused CQ WW CW 2025 TI8X dataset has historically shown this shape after
the latest workflow reload:

| Table | Expected order of magnitude |
| --- | ---: |
| `spots_flat` | 5,423 rows |
| `contest_qsos` | 3,947 rows |
| `contest_spot_matches` | 81,248 rows |
| `spotter_profiles` | hundreds of rows |
| `station_activity_5m_summaries` | hundreds of rows |
| `swpc_daily_indices` | 2 rows |
| `swpc_k_indices_3h` | 16 rows |

Exact counts can change as match policy and reload scope evolve.

## Install Or Refresh Views

Views are analyst-facing surfaces for Workbench and Tableau. Refresh them after
a clean data-directory reset or if QS starts with an external schema directory
and no persisted view definitions.

```bash
cd ~/projects/radiosport-data-lab

mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/rbn_spot_propagation_base.sql
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_qso_propagation_base.sql
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_rbn_activity_5m_base.sql
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_spot_match_base.sql
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_best_spot_match_base.sql
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/spotter_profile_base.sql
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/station_activity_5m_base.sql
```

## Reload The Focused TI8X Workflow

The workflow expects QS and `qstream-loader` to already be running. It creates
tables if needed, truncates child-first when `--reset` is supplied, reloads SWPC
context, loads focused RBN rows for one DX call or a small competitor pack,
loads public Cabrillo logs, materializes exact QSO-to-spot matches for each log,
builds station activity summaries, and refreshes views. Spotter profile
calibration is opt-in with `RBN_PROFILE_BUILD=1` while its relationship key path
is being hardened; see `SPOTTER_PROFILES_DESIGN.md` for the numeric
`spotter_node_id` follow-up. Set `RBN_STATION_ACTIVITY_BUILD=0` to skip station
activity summaries.

```bash
cd ~/projects/radiosport-data-lab

./scripts/run-ti8x-contest-workflow.sh --reset
```

Default inputs:

| Input | Default |
| --- | --- |
| RBN archives | `/tmp/rbn-data/20251129.zip`, `/tmp/rbn-data/20251130.zip` |
| Contest log | `https://cqww.com/publiclogs/2025cw/ti8x.log` |
| Station | `TI8X` |
| CTY file | `data/cty/cty.dat` |
| Loader URL | `http://127.0.0.1:8088` |
| Spotter profile build | disabled; set `RBN_PROFILE_BUILD=1` to enable |
| Station activity build | enabled for the focused station list; set `RBN_STATION_ACTIVITY_BUILD=0` to skip |

Override examples:

```bash
CONTEST_STATION=TI8X \
RBN_DAY_WORKERS=2 \
RBN_SPOT_WORKERS=4 \
./scripts/run-ti8x-contest-workflow.sh --reset
```

Small competitor-pack reload:

```bash
CONTEST_STATIONS=TI8X,V47T,8P5A \
CONTEST_LOG_URLS=https://cqww.com/publiclogs/2025cw/ti8x.log,https://cqww.com/publiclogs/2025cw/v47t.log,https://cqww.com/publiclogs/2025cw/8p5a.log \
RBN_DAY_WORKERS=2 \
RBN_SPOT_WORKERS=4 \
./scripts/run-ti8x-contest-workflow.sh --reset
```

For laptop runs where a large archive reload should create a durable checkpoint
after each daily file, run the archive phase serially and let
`rbn-archive-load` wait for the loader to drain before committing:

```bash
RBN_DAY_WORKERS=1 \
RBN_ARCHIVE_LOADER_IDLE_TIMEOUT=2m \
RBN_ARCHIVE_COMMIT_AFTER_FILE=1 \
./scripts/run-ti8x-contest-workflow.sh --reset
```

Keep the laptop RBN window narrow. The CQWW SSB planning workflow should start
with selected two-to-four-day archive sets, filtered to the target station and
peer callsigns where possible. Broad 20-plus-day analog sweeps should be treated
as AWS-scale or offline-cache work.

Use the same DX-call list for RBN archive loading and cache building. The
workflow handles that automatically through `CONTEST_STATIONS`, then loads and
matches each Cabrillo log listed in `CONTEST_LOG_URLS`. The filter is only on
the spotted DX call; all RBN spotter/receiver stations remain in the loaded
data, including important Caribbean spotters such as `TI7W`. Add `PJ4K` or
`ZF1A` when a larger Caribbean comparison pack is useful.

The workflow loads Cabrillo logs one at a time and pauses after parent rows
before sending child QSO rows. Override `RBN_PARENT_FLUSH_WAIT` only when testing
loader timing directly.

Schema activation and reset use the MySQL-compatible endpoint with `CREATE TABLE
<name>` and `TRUNCATE TABLE <name>`, so the same workflow works for single-node
and distributed runs. For single-node, start `quantastream` with `-config-dir`
pointing at this repository's `configuration` directory. For distributed mode,
start `quantastream-proxy` with `-schema-dir` pointing at the same directory.

## Tableau Notes

Use Tableau's "Other Databases (JDBC)" connector with the MySQL JDBC driver.
Manual relationships work today; automatic relationship inference through JDBC
is limited, so shipping query-ready views is the preferred analyst path.

Good first Tableau tables/views:

- `contest_best_spot_match_base`
- `contest_spot_match_base`
- `contest_qso_propagation_base`
- `rbn_spot_propagation_base`
- `spotter_profile_base`
- `station_activity_5m_base`

If Tableau gets confused during metadata discovery, enable command tracing by
starting QS with `QUANTASTREAM_MYSQL_COMMAND_TRACE=true`, reproduce the issue,
and inspect `/tmp/qs-radiosport-standard.log`.
