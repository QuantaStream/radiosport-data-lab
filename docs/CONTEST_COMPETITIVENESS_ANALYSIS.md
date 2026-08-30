# Contest Competitiveness Analysis

This note captures the first repeatable path for comparing a contest station
against a small peer set using submitted logs, RBN spots, SWPC propagation
context, and spotter calibration.

The current CQ WW CW 2025 workflow loads TI8X, V47T, and 8P5A logs plus focused
RBN spot data for the contest window. It then materializes best QSO-to-spot
matches and optional spotter profile calibration fields.

## Views

Use two analyst-facing views:

- `contest_competitiveness_qso_base`: one row per logged QSO. Use this for
  activity volume, rate, band/hour heatmaps, worked-continent mix, and category
  context.
- `contest_competitiveness_signal_base`: one row per best QSO/RBN spot match.
  Use this for received signal, receiving-continent reach, and calibrated
  signal comparisons.

Install or refresh them after the lower-level views:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_qso_propagation_base.sql
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_calibrated_spot_match_base.sql
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_competitiveness_qso_base.sql
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/contest_competitiveness_signal_base.sql
```

## Signal Metrics

`raw_signal_db` is the RBN signal report for the best spot matched to a logged
QSO. It is useful, but strong receiving stations can make every signal look
better.

`normalized_signal_db` subtracts the receiving station's baseline signal. This
is a first cut at answering: "Was this station loud relative to what this
spotter usually hears?"

`calibrated_reach_numerator` is `normalized_signal_db * spotter_weight`.
`calibrated_reach_weight` is the denominator weight. Build the Tableau
calculated field as:

```text
SUM([calibrated_reach_numerator]) / SUM([calibrated_reach_weight])
```

That keeps the weighted SNR correct at whatever grain Tableau is showing: band,
hour, station, or receiving continent.

This metric is directional, not a final score. It helps separate "I was heard
by a few very strong skimmers" from "I was broadly hearable by ordinary
receivers."

## Tableau Worksheets

Start with `contest_competitiveness_signal_base`:

- Band reach: put `band` on Rows, `station_call` on Color, and the calculated
  `Calibrated Reach SNR` on Columns.
- Receiving-continent reach: put `receiving_continent` on Rows, `band` on
  Columns, `Calibrated Reach SNR` on Color, and `COUNTD(qso_id)` or
  `COUNT(match_id)` on Label.
- Hour-by-band reach: put `qso_hour` on Columns, `band` on Rows,
  `Calibrated Reach SNR` on Color, and filter by `station_call`.
- Spotter concentration: put `spotter_call` on Rows, `COUNT(match_id)` on
  Columns, and filter by station and band.

Use `contest_competitiveness_qso_base` for activity volume:

- Rate by hour: put `qso_hour` on Columns, `COUNTD(qso_id)` on Rows, and
  `station_call` on Color.
- Band plan: put `qso_hour` on Columns, `band` on Rows, and `COUNTD(qso_id)`
  on Color or Label.
- Worked-continent mix: put `worked_continent` on Rows, `band` on Columns, and
  `COUNTD(qso_id)` on Color.

Do not compare logged QSO volume by `receiving_continent`; receiving continent
only exists in RBN spot observations. Use the signal view when the receiver
location matters.

## Early Read From Current Sample

In the first TI8X, V47T, and 8P5A CQ WW CW 2025 slice:

- TI8X looks competitive on 15m, 20m, 40m, and 80m by calibrated reach.
- 10m is mixed; V47T appears stronger by calibrated reach in the current
  sample.
- 160m needs caution. TI8X has very few matched observations there, so one or
  two decisions can dominate the shape.
- The other stations have much higher total QSO volume. Signal quality and
  contest rate are related, but they are not the same measure.

The next useful product step is missed-opening analysis: compare each station's
five-minute activity buckets against peer stations and mark buckets where peers
were broadly heard but the target station was absent or weak.
