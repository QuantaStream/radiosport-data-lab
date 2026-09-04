#!/usr/bin/env bash
set -Eeuo pipefail

app_repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
work_dir="${CQWW_BATTLE_WORK_DIR:-/tmp/radiosport-cqww-cw-2025-battle}"
source_dir="$work_dir/sources"
output_dir="${CQWW_BATTLE_OUTPUT_DIR:-$work_dir/output}"
refresh=0

if [ "${1:-}" = "--refresh" ]; then
  refresh=1
  shift
fi
if [ "$#" -ne 0 ]; then
  echo "usage: run-cqww-cw-2025-battle.sh [--refresh]" >&2
  exit 2
fi

mkdir -p "$source_dir" "$output_dir"

fetch() {
  local url="$1" path="$2" tmp
  if [ "$refresh" = "0" ] && [ -s "$path" ]; then
    return
  fi
  tmp="${path}.tmp"
  rm -f "$tmp"
  curl --fail --location --silent --show-error \
    --user-agent 'radiosport-data-lab/0.1' \
    --output "$tmp" "$url"
  mv "$tmp" "$path"
}

ef8r_url="https://cqww.com/publiclogs/2025cw/ef8r.log"
cq9a_url="https://cqww.com/publiclogs/2025cw/cq9a.log"
fivej1dx_url="https://cqww.com/publiclogs/2025cw/5j1dx.log"
cty_url="https://www.country-files.com/cty/download/3537/cty-3537.zip"
rules_url="https://cqww.com/rules/2025_rules_cqww.pdf"
results_url="https://cqww.com/results/2025_cq_ww_dx_cw_results.pdf"

fetch "$ef8r_url" "$source_dir/ef8r.log"
fetch "$cq9a_url" "$source_dir/cq9a.log"
fetch "$fivej1dx_url" "$source_dir/5j1dx.log"
fetch "$cty_url" "$source_dir/cty-3537.zip"
fetch "$rules_url" "$source_dir/2025_rules_cqww.pdf"
fetch "$results_url" "$source_dir/2025_cq_ww_dx_cw_results.pdf"

for log_path in "$source_dir/ef8r.log" "$source_dir/cq9a.log" "$source_dir/5j1dx.log"; do
  grep -q '^QSO:' "$log_path" || {
    echo "downloaded source contains no Cabrillo QSO rows: $log_path" >&2
    exit 1
  }
done
unzip -tq "$source_dir/cty-3537.zip" >/dev/null
grep -q '^%PDF-' "$source_dir/2025_rules_cqww.pdf"
grep -q '^%PDF-' "$source_dir/2025_cq_ww_dx_cw_results.pdf"

SOURCE_DIR="$source_dir" LOCK_PATH="$app_repo/docs/cases/cqww-cw-2025-battle/source-lock.json" \
python3 - <<'PY'
import hashlib
import json
import os
from pathlib import Path

source_dir = Path(os.environ["SOURCE_DIR"])
lock = json.loads(Path(os.environ["LOCK_PATH"]).read_text(encoding="utf-8"))
errors = []
for expected in lock["sources"]:
    path = source_dir / expected["name"]
    data = path.read_bytes()
    actual_hash = hashlib.sha256(data).hexdigest()
    if len(data) != expected["bytes"] or actual_hash != expected["sha256"]:
        errors.append(
            f"{expected['name']}: expected {expected['bytes']} bytes/{expected['sha256']}, "
            f"got {len(data)} bytes/{actual_hash}"
        )
if errors:
    raise SystemExit("source-lock verification failed:\n" + "\n".join(errors))
PY

rm -rf "$work_dir/cty-3537"
mkdir -p "$work_dir/cty-3537"
unzip -q "$source_dir/cty-3537.zip" cqww.dat -d "$work_dir/cty-3537"

(
  cd "$app_repo"
  go run ./cmd/cqww-battle-report \
    -cty-dat "$work_dir/cty-3537/cqww.dat" \
    -out-dir "$output_dir" \
    "$source_dir/ef8r.log" \
    "$source_dir/cq9a.log" \
    "$source_dir/5j1dx.log"
)

OUTPUT_DIR="$output_dir" python3 - <<'PY'
import csv
import os
from pathlib import Path

root = Path(os.environ["OUTPUT_DIR"])
summaries = {row["station"]: row for row in csv.DictReader((root / "summaries.csv").open())}
expected = {
    "EF8R": (12991, 12708, 283, 37993, 561, 168, 27696897, 27582918),
    "CQ9A": (11520, 11340, 180, 33911, 559, 167, 24619386, 24689392),
    "5J1DX": (9833, 9218, 615, 27310, 501, 161, 18079220, 18024600),
}
fields = ("submitted_qsos", "counted_qsos", "duplicates", "qso_points", "countries", "zones", "reconstructed_score", "claimed_score")
for call, values in expected.items():
    actual = tuple(int(summaries[call][field]) for field in fields)
    if actual != values:
        raise SystemExit(f"summary regression for {call}: expected {values}, got {actual}")

changes = list(csv.DictReader((root / "lead-changes.csv").open()))
expected_leaders = ["EF8R", "CQ9A", "EF8R", "CQ9A", "EF8R", "CQ9A", "EF8R"]
actual_leaders = [row["to_leader"] for row in changes]
if actual_leaders != expected_leaders:
    raise SystemExit(f"lead sequence regression: expected {expected_leaders}, got {actual_leaders}")
PY

SOURCE_DIR="$source_dir" OUTPUT_DIR="$output_dir" \
EF8R_URL="$ef8r_url" CQ9A_URL="$cq9a_url" FIVEJ1DX_URL="$fivej1dx_url" CTY_URL="$cty_url" \
RULES_URL="$rules_url" RESULTS_URL="$results_url" \
python3 - <<'PY'
import datetime
import hashlib
import json
import os
from pathlib import Path

source_dir = Path(os.environ["SOURCE_DIR"])
output_dir = Path(os.environ["OUTPUT_DIR"])
items = [
    ("ef8r.log", os.environ["EF8R_URL"]),
    ("cq9a.log", os.environ["CQ9A_URL"]),
    ("5j1dx.log", os.environ["FIVEJ1DX_URL"]),
    ("cty-3537.zip", os.environ["CTY_URL"]),
    ("2025_rules_cqww.pdf", os.environ["RULES_URL"]),
    ("2025_cq_ww_dx_cw_results.pdf", os.environ["RESULTS_URL"]),
]
sources = []
for name, url in items:
    data = (source_dir / name).read_bytes()
    sources.append({
        "name": name,
        "url": url,
        "bytes": len(data),
        "sha256": hashlib.sha256(data).hexdigest(),
    })
lock = {
    "case_id": "cqww-cw-2025-soab-hp-battle",
    "generated_at": datetime.datetime.now(datetime.timezone.utc).isoformat().replace("+00:00", "Z"),
    "sources": sources,
}
(output_dir / "source-lock.json").write_text(json.dumps(lock, indent=2) + "\n", encoding="utf-8")
PY

if [ -n "${CQWW_BATTLE_LOADER_URL:-}" ]; then
  OUTPUT_DIR="$output_dir" \
  LOADER_URL="$CQWW_BATTLE_LOADER_URL" \
  python3 - <<'PY'
import json
import os
import urllib.request
from pathlib import Path

url = os.environ["LOADER_URL"].rstrip("/") + "/ingest/json"
root = Path(os.environ["OUTPUT_DIR"])
for path in (root / "battle-timeline.jsonl", root / "rbn-reach-region.jsonl"):
    if not path.exists():
        continue
    events = [json.loads(line) for line in path.open(encoding="utf-8")]
    for start in range(0, len(events), 500):
        batch = events[start:start + 500]
        payload = json.dumps({"events": batch}).encode()
        request = urllib.request.Request(url, data=payload, headers={"Content-Type": "application/json"})
        with urllib.request.urlopen(request, timeout=60) as response:
            result = json.load(response)
        if result.get("failed", 0) or result.get("accepted", 0) != len(batch):
            raise SystemExit(f"loader rejected {path.name} batch at {start}: {result}")
    print(f"loaded {len(events)} events from {path.name}")
PY
fi

printf 'Battle outputs: %s\n' "$output_dir"
