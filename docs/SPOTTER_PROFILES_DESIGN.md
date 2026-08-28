# Spotter Profiles Design

This note captures the design for calibrating Reverse Beacon Network spotter
stations before using their signal reports in contest analysis.

## Motivation

Raw RBN spots answer a simple question: a spotter heard a station at a reported
signal level. For contest analysis, that is not quite enough. Some skimmers have
excellent antennas, quiet sites, or unusually broad receive coverage. If those
stations dominate a result set, a station may look broadly audible even when the
evidence is coming from a small number of highly capable receivers.

The product question is:

```text
Could ordinary stations in the target region hear me, or was I only visible to
the loudest skimmers?
```

The answer needs both raw observations and a calibration layer for spotter
behavior.

## Proposed Tables

### `rbn_spotter_nodes`

`rbn_spotter_nodes` stores RBN-published metadata about skimmer nodes. It should
be loaded from periodic snapshots of the RBN node pages or any future official
machine-readable feed.

| Column | Mapper | Notes |
| --- | --- | --- |
| `spotter_call` | `StringLexBSI length=8 maxLen=16` | Primary key. |
| `spotter_prefix` | `StringEnum` | CTY/DXCC prefix for the node. |
| `spotter_continent` | `StringEnum` | Node continent. |
| `grid` | `StringLexBSI length=8 maxLen=16` | Maidenhead grid when available. |
| `dxcc_id` | `IntBSI` | DXCC numeric identifier when available. |
| `country_name` | `StringEnum` | Human-readable country. |
| `cq_zone` | `IntBSI` | CQ zone. |
| `itu_zone` | `IntBSI` | ITU zone. |
| `bands` | `StringEnum` | Published band summary. |
| `skimmer_software` | `StringEnum` | Software/version if published. |
| `aggregator_software` | `StringEnum` | Aggregator/version if published. |
| `first_seen_at` | `TimestampBSI` | RBN first-seen timestamp if published. |
| `last_seen_at` | `TimestampBSI` | RBN last-seen timestamp if published. |
| `source` | `StringEnum` | Source label, such as `rbn_nodes`. |
| `loaded_at` | `TimestampBSI` | Snapshot load time. |

This is reference metadata. It should not block live spot ingestion.

### `spotter_profile_snapshots`

`spotter_profile_snapshots` stores computed behavior for a spotter over a fixed
analysis window. The first window can be the currently loaded contest/archive
range. Later windows can be daily, weekly, rolling 30 day, contest-specific, or
band-specific.

| Column | Mapper | Notes |
| --- | --- | --- |
| `profile_id` | `StringLexBSI length=16 maxLen=96` | Stable key: spotter/window/profile kind. |
| `spotter_call_ref` | `ParentRelation -> rbn_spotter_nodes` | Relationship to node metadata when present. |
| `spotter_call` | `StringLexBSI length=8 maxLen=16` | Query-friendly copy. |
| `profile_kind` | `StringEnum` | `contest`, `daily`, `rolling_30d`, etc. |
| `window_start` | `TimestampBSI` | Inclusive UTC window start. |
| `window_end` | `TimestampBSI` | Exclusive UTC window end. |
| `band` | `StringEnum` | Optional band-specific profile, or `ALL`. |
| `mode` | `StringEnum` | Optional mode-specific profile, or `ALL`. |
| `total_spots` | `IntBSI` | Total spots in the window. |
| `active_days` | `IntBSI` | Distinct active UTC days. |
| `active_hours` | `IntBSI` | Distinct active UTC hours. |
| `distinct_dx_calls` | `IntBSI` | Breadth of heard stations. |
| `distinct_dx_prefixes` | `IntBSI` | Geographic breadth. |
| `avg_signal_db` | `FloatScaleBSI scale=2` | Mean reported signal. |
| `min_signal_db` | `IntBSI` | Minimum reported signal. |
| `max_signal_db` | `IntBSI` | Maximum reported signal. |
| `p50_signal_db` | `FloatScaleBSI scale=2` | Median, computed offline. |
| `p90_signal_db` | `FloatScaleBSI scale=2` | High-side signal reference. |
| `volume_weight` | `FloatScaleBSI scale=6` | Weight derived from total spot volume. |
| `normalization_offset_db` | `FloatScaleBSI scale=2` | Baseline value for normalized SNR. |
| `profile_quality` | `StringEnum` | `good`, `sparse`, `stale`, `unknown`. |
| `computed_at` | `TimestampBSI` | Job execution time. |

### `spotter_profiles`

`spotter_profiles` is the current effective profile per spotter. It is the table
that views and Tableau should join. A profile build can replace this table from
the latest accepted snapshot, or update it in place.

| Column | Mapper | Notes |
| --- | --- | --- |
| `spotter_call` | `StringLexBSI length=8 maxLen=16` | Primary key. |
| `spotter_prefix` | `StringEnum` | From node metadata or CTY parser. |
| `spotter_continent` | `StringEnum` | From node metadata or CTY parser. |
| `grid` | `StringLexBSI length=8 maxLen=16` | Optional node grid. |
| `total_spots` | `IntBSI` | Current calibration volume. |
| `active_days` | `IntBSI` | Active-day count. |
| `avg_signal_db` | `FloatScaleBSI scale=2` | Mean spotter signal report. |
| `p50_signal_db` | `FloatScaleBSI scale=2` | Median spotter signal report. |
| `p90_signal_db` | `FloatScaleBSI scale=2` | High-signal reference. |
| `spotter_weight` | `FloatScaleBSI scale=6` | Down-weighting factor for dominant spotters. |
| `profile_quality` | `StringEnum` | Profile readiness label. |
| `profile_computed_at` | `TimestampBSI` | Current profile timestamp. |

## Weighting Options

The simplest useful weighting is volume based:

```text
spotter_weight = 1 / sqrt(total_spots)
weighted_snr = sum(signal_db * spotter_weight) / sum(spotter_weight)
```

That prevents one very active skimmer from overwhelming the result. It is easy
to compute, easy to explain, and stable enough for the first slice.

A softer alternative is:

```text
spotter_weight = 1 / log(2 + total_spots)
```

This penalizes high-volume stations less aggressively.

The more interesting long-term metric is normalized SNR:

```text
normalized_snr = signal_db - spotter_baseline_signal_db
```

Where `spotter_baseline_signal_db` can start as `avg_signal_db` and later move
to `p50_signal_db` or a band/mode-specific baseline. This asks whether the
station was loud relative to what that spotter normally hears.

## Reach Metrics

Weighted SNR should not stand alone. Broad audibility is better represented by
combining strength with diversity:

| Metric | Meaning |
| --- | --- |
| `distinct_spotters` | Independent receiving stations. |
| `distinct_spotter_prefixes` | Geographic spread within continents. |
| `distinct_spotter_continents` | Intercontinental reach. |
| `weighted_snr` | Strength adjusted for spotter dominance. |
| `avg_normalized_snr` | Strength relative to spotter baseline. |
| `reach_score` | Composite score for Tableau and planning views. |

An initial reach score could be:

```text
reach_score =
  distinct_spotters
  * log(1 + distinct_spotter_prefixes)
  * greatest(avg_normalized_snr, 1)
```

The exact formula is product policy, not storage policy. Keep the base metrics
materialized so formulas can evolve.

## Builder Job

Add a command such as:

```bash
go run ./cmd/spotter-profile-build \
  -target http://127.0.0.1:8088/ingest/json \
  -from 2025-11-29 \
  -to 2025-12-01 \
  -profile-kind contest \
  -source-table spots_flat
```

The first implementation can:

1. Query `spots_flat` through QS or read a parsed archive cache.
2. Group by `spotter_call`, with optional band/mode windows.
3. Compute volume, active days/hours, signal summary, and weights.
4. Emit `rbn_spotter_nodes` stubs for unknown spotters.
5. Emit `spotter_profile_snapshots`.
6. Emit or update `spotter_profiles`.

For high-volume archive runs, reading parsed archive files or a cache may be
faster than querying QS for all raw rows. For correctness, the profile job should
record its source window and source files so results are reproducible.

## Views

Add a Tableau-facing view after the profile tables exist:

```text
contest_weighted_spot_match_base
```

The view should start from `contest_best_spot_match_base` and join
`spotter_profiles` by `spotter_call`. It should expose:

| Column | Meaning |
| --- | --- |
| `signal_db` | Raw RBN signal report. |
| `spotter_weight` | Current calibration weight. |
| `weighted_signal_db` | `signal_db * spotter_weight`. |
| `spotter_avg_signal_db` | Spotter baseline. |
| `normalized_signal_db` | Raw signal minus baseline. |
| `profile_quality` | Whether the calibration is trustworthy. |

Useful Tableau aggregate fields:

```sql
select
  qso_hour,
  band,
  spotter_continent,
  avg(signal_db) as avg_snr,
  sum(signal_db * spotter_weight) / sum(spotter_weight) as weighted_snr,
  avg(signal_db - spotter_avg_signal_db) as avg_normalized_snr,
  count(*) as qsos
from contest_weighted_spot_match_base
group by qso_hour, band, spotter_continent
order by qso_hour, band, qsos desc;
```

If QS does not yet support every expression Tableau emits for this shape, the
builder can materialize `weighted_signal_db` and `normalized_signal_db` into a
match summary table.

## Product Notes

- Do not put profile lookups in the live telnet hot path.
- Profile building is an offline or nearline analytics job.
- Missing profiles should default to `spotter_weight=1.0` and
  `profile_quality='unknown'`.
- RBN node metadata is useful but not authoritative; computed behavior should be
  the primary calibration source.
- Band-specific profiles will likely matter. A station may be ordinary on one
  band and exceptional on another.
- This design is also a useful QuantaStream product story: raw streaming facts,
  compact bitmap-native identifiers, relationship-vector joins, and derived
  analyst views.

## Open Questions

- Should `spotter_profiles` be one current row per spotter, or one current row
  per spotter/band/mode?
- Should the first weighting formula use square root or logarithmic damping?
- Should normalized SNR use average, median, or band-specific median as the
  baseline?
- Do we need separate profiles for contest periods versus ordinary daily
  traffic?
- Should RBN node snapshots be fetched directly by the app at startup, by a
  scheduled job, or only on demand?
