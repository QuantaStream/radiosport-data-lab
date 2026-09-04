# CQ WW CW 2025: The World SOAB HP Battle

This case study reconstructs the 48-hour CQ World Wide DX CW contest battle
among Dan Craig, N6MJ, operating EF8R; Chris Hurlbut, KL9A, operating CQ9A;
and Emir "Braco" Memic, E77DX, operating 5J1DX.

Read the community-facing result in [`findings.md`](findings.md). Stage 2
begins in [`article-comparison.md`](article-comparison.md), which separates the
official results article's claims from the contemporaneous 3830 accounts and
lists the log/RBN tests needed to corroborate the latter.

The objective is to produce a reproducible, evidence-backed account of how the
contest developed, not merely a visualization of the final standings. The
analysis should expose score composition, lead changes, operating choices,
propagation, signal reach, and log-checking effects at useful time grains.

## Evidence policy

Every published finding must be classified as one of:

- **reported**: stated in a contemporary account or operator interview;
- **confirmed**: a reported claim independently reproduced from public data;
- **derived**: calculated from public data and not located in prior coverage;
- **inferred**: an interpretation supported by evidence but not directly
  observable; or
- **unknown**: a question the public evidence cannot resolve.

Novelty is a research result, not an assumption. Before labeling a result
derived or new, search the sources in `sources.yaml` and record the comparison
in the eventual findings ledger. Avoid claims about operator intent unless an
operator said it or clearly label them as inferred.

## Reproducibility contract

The finished workflow should:

1. Fetch or accept locally cached copies of every public source.
2. Record the source URL, retrieval time, byte count, and SHA-256 digest.
3. Preserve raw files outside Git and write generated outputs to an ignored
   runtime directory.
4. Load the three Cabrillo logs, focused RBN observations, and SWPC context
   through the existing RadioSport Data Lab and QuantaStream paths.
5. Reconstruct CQ WW score state after every QSO and at five-minute intervals.
6. Validate raw reconstructed totals against submitted claimed scores.
7. Validate published result totals against the official CQ WW results.
8. Emit tables behind every chart so findings remain inspectable without a BI
   tool or AI subscription.

## Initial questions

- Precisely when did the lead change, and which bands or multiplier runs
  produced each change?
- How much of EF8R's margin came from QSO points, countries, and zones?
- Did CQ9A's nearly identical final multiplier count conceal materially
  different multiplier timing?
- Which paths and bands gave each geography a temporary advantage?
- Can RBN timing distinguish sustained run operation, band changes, and
  plausible interleaved-radio operation without overclaiming intent?
- When did submitted-log errors accumulate, and did their rate change with
  elapsed operating time?
- How faithfully did the online scoreboard represent the underlying race?
- Which observations would have predicted the winner at 12, 24, and 36 hours?

## Published validation anchors

| Station | Operator | Submitted QSOs | Claimed score | Final QSOs | Final score |
| --- | --- | ---: | ---: | ---: | ---: |
| EF8R | N6MJ | 12,991 | 27,582,918 | 12,552 | 26,516,025 |
| CQ9A | KL9A | 11,520 | 24,689,392 | 11,219 | 23,779,792 |
| 5J1DX | E77DX | 9,833 | 18,024,600 | 9,005 | 16,738,525 |

Submitted values come from the public Cabrillo files. Final values come from
the official results article. These are validation targets, not substitutes
for independently reconstructing the scoring timeline.

## Planned outputs

- `source-lock.json`: retrieval metadata and content hashes;
- `score-timeline.csv`: cumulative points, countries, zones, and score;
- `battle-timeline.jsonl`: the aligned timeline in QuantaStream loader format;
- `battle-timeline.csv`: aligned five-minute station state and lead margins;
- `band-activity.csv`: activity, rate, points, and multipliers by band/time;
- `rbn-reach.csv`: calibrated reception evidence by band, time, and region;
- `rbn-reach-region.csv`: the same reduction split by receiver continent;
- `rbn-reach-region.jsonl`: regional reach evidence in QuantaStream loader format;
- `rbn-matched-skimmers.csv`: same-receiver EF8R/CQ9A SNR comparisons;
- `rbn-high-band-daily.csv`: reproducible log/RBN day-level alignment;
- `three-hour-checkpoints.csv`: score-race checkpoints for turning-point work;
- `findings.md`: sourced claims with evidence classification and uncertainty;
- a static, shareable report with links back to the underlying tables.

## Reproduce the log-only dataset

Run from the repository root:

```bash
./scripts/run-cqww-cw-2025-battle.sh
```

The script downloads the three raw public logs and CTY-3537, the country-file
release published immediately before the contest. It writes raw sources under
`/tmp/radiosport-cqww-cw-2025-battle/sources` and generated CSV files plus
`source-lock.json` under `/tmp/radiosport-cqww-cw-2025-battle/output`. Set
`CQWW_BATTLE_WORK_DIR` or `CQWW_BATTLE_OUTPUT_DIR` to use another runtime
location. Pass `--refresh` to fetch fresh copies and replace the local cache.

The reconstruction uses `cqww.dat` from CTY-3537. Its independently calculated
scores are expected to differ slightly from the `CLAIMED-SCORE` fields because
the operators' logging programs may have used different country-file releases
or local callsign exceptions. Both values and their delta are retained in
`summaries.csv`; the claimed value is never used to alter the reconstruction.

To load the timeline into an already running QuantaStream loader configured
with this repository's `configuration` directory, create the battle and RBN
reach tables and set the loader base URL. If the RBN JSONL has already been
generated, the same run loads both datasets:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  -e 'create table cqww_battle_buckets; create table cqww_rbn_reach_buckets'
CQWW_BATTLE_LOADER_URL=http://127.0.0.1:8088 \
  ./scripts/run-cqww-cw-2025-battle.sh
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/queries/cqww_cw_2025_battle.sql
```

The optional upload is deliberately off by default, keeping the normal
workflow reproducible without requiring a running database.

## Reduce the RBN archives

Download the official `20251129.zip` and `20251130.zip` daily archives from
the [RBN raw-data page](https://www.reversebeacon.net/raw_data/), verify them
against [`rbn-source-lock.json`](rbn-source-lock.json), then run:

```bash
go run ./cmd/cqww-rbn-report \
  -band-activity /tmp/radiosport-cqww-cw-2025-battle/output/band-activity.csv \
  -output-dir /tmp/radiosport-cqww-cw-2025-battle/output \
  /path/to/20251129.zip /path/to/20251130.zip
```

The reducer streams both archives once, retains exact-call observations for
the three competitors, and writes reach, regional reach, matched-skimmer, and
daily high-band outputs. The regional JSONL can be loaded into
`cqww_rbn_reach_buckets` by rerunning the battle workflow with
`CQWW_BATTLE_LOADER_URL` set. The matched-skimmer output compares EF8R and CQ9A
only at the same receiver, band, and five-minute interval, with separate
continent and all-receiver rows.
