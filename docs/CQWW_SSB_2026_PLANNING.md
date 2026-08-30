# CQWW SSB 2026 Planning

This note captures the first planning shape for CQ World Wide SSB 2026. The
goal is to make the analysis useful to contesters without requiring a BI tool,
while still keeping the data model friendly to QuantaStream, SQL clients, and
Codex-guided exploration.

## Planning Question

For a Caribbean or Central America single-op, high-power, non-assisted entry:

- What did comparable stations do in similar conditions?
- Which hours and bands deserve protected operating time?
- Which multiplier zones need intentional hunting windows?
- Which openings were likely available but missed?
- Did station signal reach, operator choices, or propagation drive the result?

## Data Separation

Use public logs for contest behavior:

- rate by hour and band
- band changes and interleaving
- multiplier timing
- worked-zone coverage
- peer station comparison

Use RBN plus SWPC for propagation evidence:

- path availability
- quiet versus disturbed Kp buckets
- 10m/15m/20m/40m seasonality
- receiving-region coverage
- calibrated signal reach

RBN is after-the-fact evidence. It is not a contest-time aid for non-assisted
operation, but it is valuable for postmortem and future planning.

## SFI Anchors

As of 2026-08-30, NOAA/SWPC predicts October 2026 F10.7 near 133 and November
2026 F10.7 near 132. CQWW SSB is a late-October contest, so October is the
better planning anchor.

The first historical SFI anchors are:

| Year | CQWW SSB weekend | Sat SFI | Sun SFI | Weekend avg | Use |
| --- | --- | ---: | ---: | ---: | --- |
| 2025 | 2025-10-25/26 | 127 | 124 | 125.5 | direct peer baseline |
| 2023 | 2023-10-28/29 | 128 | 135 | 131.5 | forecast-like analog |
| 2022 | 2022-10-29/30 | 134 | 131 | 132.5 | forecast-like analog |
| 2011 | 2011-10-29/30 | 123 | 127 | 125.0 | close to 2025 |
| 2015 | 2015-10-31/11-01 | 119 | 124 | 121.5 | slightly lower analog |
| 2004 | 2004-10-30/31 | 136 | 139 | 137.5 | high side analog |

The first pass should not overfit to one number. Treat SFI as a window, then
use K/Kp, seasonality, and actual DX activity to choose study periods.

## Contest Log Packs

Peer baseline for CQWW SSB 2025:

- `TI8X`
- `8P5A`
- `HP3/VE3DZ`

`V47T` has a 2025 Phone public log, but it is multi-op assisted and should not
be used as a direct single-op peer for this analysis.

Forecast-like analog pack for CQWW SSB 2023:

- `8P5A`
- `6Y1V`
- `KP2M`

Additional stations should be selected by category and geography, not only by
claimed score. A good peer candidate is single-op, non-assisted, all-band,
high-power, one transmitter, and located in the Caribbean/Central America
neighborhood.

Guy did not operate CQWW SSB in 2023, so this pack is a propagation and peer
behavior analog rather than a direct self-versus-peer comparison. If a future
analysis uses slashed callsigns, keep the exact value as the radio callsign in
filters, Cabrillo parsing, and QS rows. Do not derive public-log URLs or local
filenames from slash callsigns; use an explicit local path or an exact link
copied from the public log index.

Do not treat bare `N7ZG` RBN hits as Guy's Caribbean/Central America operating
evidence for this analysis. Guy used bare `N7ZG` from Washington years earlier;
current contest-planning comparisons should require the exact operating call,
contest date, and operating location to match the analysis being performed.

## RBN Propagation Window Rules

The broad RBN candidate search window is:

```text
October 20 through November 15
```

Widen only if the candidate set is too thin:

```text
October 15 through November 20
```

Do not load that whole window into a laptop QS store as the default workflow.
Use it to discover candidate days, then load a narrow set of selected RBN days.
The laptop-first RBN shape is two to four days at a time, filtered to the target
station and selected peers when possible. A full 20-plus-day RBN sweep belongs
in a larger machine, AWS, or an offline cache/profiling pass.

Use `cmd/rbn-archive-scan` before loading archives:

```bash
go run ./cmd/rbn-archive-scan \
  -dx-calls '8P5A,6Y1V,V47T,HP3/VE3DZ,KP2M,PJ2T,P40W' \
  /tmp/rbn-data/2023*.zip
```

The first laptop seed for 2023 analog RBN work is:

```text
2023-10-22, 2023-10-24, 2023-10-25, 2023-11-14
```

Those dates were selected from the initial focused archive scan because they had
actual `8P5A` RBN visibility inside the analog range. Replace them as better
candidate days emerge.

The first 2023 scan found useful `8P5A` visibility, no staged-window RBN spots
for `6Y1V`, and strong `KP2M` visibility on 2023-10-31, 2023-11-04,
2023-11-05, and 2023-11-15. That means `6Y1V` remains useful as a public-log
contest peer, while RBN propagation comparisons may need a different same-region
station or a wider discovered archive set.

The next laptop analog pack should focus on:

```text
RBN stations: 8P5A, KP2M
RBN days: 2023-10-22, 2023-11-04, 2023-11-05
Contest logs: 8P5A, 6Y1V, KP2M
```

This keeps the row count modest, preserves one known direct peer in the public
logs, and adds a high-visibility Caribbean/Virgin Islands station for RBN
propagation comparison.

A candidate window should score well on:

- SFI closeness to roughly 120-140, with 130-135 preferred for 2026 planning
- explicit Kp buckets: quiet, unsettled, and disturbed
- high non-beacon DX spot volume
- high unique DX calls
- high unique spotter calls
- useful 10m and 15m activity
- broad 20m and 40m coverage
- Caribbean/Central America stations visible in the RBN data
- JA/Asia, Pacific, Africa, and hard-zone path evidence

Avoid windows dominated by beacon or NCDXF traffic. The goal is contest-like DX
activity, not just RF noise in the archive.

## First Reports

The first generated reports should be useful without Tableau:

- station summary: category, claimed score, QSO count, band split
- hourly rate by station and band
- target multiplier-zone coverage by station and band
- high-band opportunity report for 10m and 15m
- SWPC context by contest hour
- calibrated RBN signal reach by station, band, and receiving continent
- missed-opening candidates where competitors were active but the target
  station was not

The report output should be Markdown or static HTML first. CSV extracts are
useful sidecars for spreadsheet and visualization users.

## First Workflow

The first runnable workflow should:

1. Load SWPC data for 2025, 2023, and nearby analog windows.
2. Load the CQWW SSB 2025 peer logs: `TI8X`, `8P5A`, and `HP3/VE3DZ`.
3. Load the CQWW SSB 2023 analog logs: `8P5A` and `6Y1V`.
4. Load a small selected-day RBN analog set, not the whole broad window.
5. Install the contest propagation and competitiveness views.
6. Run the CQWW SSB planning query pack.
7. Emit a Markdown report with links to generated CSV outputs.

RBN analog-window discovery is the next layer after the log/SWPC workflow is
repeatable.

When running the archive loader on a laptop, prefer the safe serialized path:

```bash
./scripts/run-cqww-ssb-2026-planning.sh --reset
```

## Implementation Notes

- Keep Go as the core implementation language for parsing, caching, loading,
  and report generation.
- Keep Python optional for notebooks or chart experiments.
- Keep Tableau optional. It is valuable for QS compatibility testing and
  exploratory visualization, but it should not be required for community use.
- Do not commit RBN archives, Cabrillo logs, QRZ-derived data, QS runtime data,
  or generated reports.
- SWPC files are small public U.S. government datasets and may be committed as
  a convenience cache.

## Source References

- CQWW public logs: `https://cqww.com/publiclogs/`
- Reverse Beacon Network archive: `https://reversebeacon.net/raw_data/`
- NOAA SWPC services: `https://services.swpc.noaa.gov/`
