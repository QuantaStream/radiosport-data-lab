# Stage 2: Published Narrative Versus Public Evidence

## Purpose

The official results article calls EF8R versus CQ9A an epic, hard-fought
48-hour race and reports the winner, final scores, records, and best hourly
rates. It does not explain when the race was decided or why the three stations
separated. This companion analysis tests that missing mechanism using:

1. contemporaneous claims from 3830 and the online scoreboard;
2. an independent reconstruction from the public raw Cabrillo logs and RBN;
3. the checked totals and retrospective narrative in the official article.

Operator statements establish what an operator reported, not the underlying
cause. Adjudication is visible only in aggregate because contact-level
checking reports are not public here.

## What the results article establishes

The March 2026 [official results article][official] establishes that EF8R
(N6MJ) won World Single Operator High Power with 26,516,025 points; CQ9A
(KL9A) was second with 23,779,792 and also exceeded the previous world record;
and 5J1DX (E77DX) was third with 16,738,525. It gives best 60-minute rates of
384, 351, and 328 respectively.

The article supplies no lead chronology, band-by-band explanation, or account
of the operational events behind the result. Stage 1 adds the first two: six
reconstructed lead changes in the opening 6h05 and a decisive EF8R production
advantage on 10 and 15 meters.

## Claimed versus checked

The contemporaneous [3830 summary][3830-summary] identifies all three entries
as 2BSIQ. These are post-contest claimed results, not the evolving live score
and not checked scores.

| Station | 3830 net QSOs | 3830 claimed | Official checked | Reduction | Reduction |
| --- | ---: | ---: | ---: | ---: | ---: |
| EF8R | 12,708 | 27,582,918 | 26,516,025 | 1,066,893 | 3.87% |
| CQ9A | 11,340 | 24,687,208 | 23,779,792 | 907,416 | 3.68% |
| 5J1DX | 9,218 | 18,024,600 | 16,738,525 | 1,286,075 | 7.14% |

The EF8R-CQ9A margin narrowed from 2,895,710 claimed points to 2,736,233
checked points, a change of 159,477. Adjudication changed the size, but not the
identity, of the winner. 5J1DX had nearly twice the percentage score reduction
of either zone-33 entry. Aggregate totals cannot identify busted calls,
exchanges, NILs, or penalties.

The official band table reports 12,552 checked QSOs for EF8R, 11,219 for CQ9A,
and 9,005 for 5J1DX. Relative to the 3830 net totals, the differences are 156,
121, and 213 respectively. These are context, not a reconstruction of checking.

## What the operators reported

### EF8R: a short fatigue interruption

In his [3830 account][3830-ef8r], N6MJ reported an extreme fatigue period
during approximately 09:00--12:00 UTC Sunday, followed by about 20 minutes
off the air for a shower. He also reported that pre-contest European receive
noise disappeared during the event.

**Test:** locate the interruption in the raw log; measure score production
immediately before and after it; use RBN spots across multiple skimmers to
distinguish silence from search-and-pounce or a logging gap; and determine
whether CQ9A materially reduced the lead.

### CQ9A: keyboard and CW synchronization trouble

In his [3830 account][3830-cq9a], KL9A reported that each keyboard could move
to the other log window and stop CW transmission. He struggled with the
problem for the first few hours and learned how to stay synchronized by about
hour 40. He also believed he spent too much time on 160 meters Sunday.

**Test:** search for anomalous short gaps, rate volatility, band-change
patterns, or reduced two-radio interleaving during the opening hours. Compare
the second-night 160-meter gain with EF8R's gain on other bands. RBN can show
when CQ9A transmitted but cannot directly identify a keyboard failure.

### 5J1DX: station readiness, amplifier failures, and QRN

In his [3830 account][3830-5j1dx], E77DX described a late station build,
imperfect inter-band isolation, a failed 15-meter high-power filter, and
compromised receive antennas. He reported a PC power interruption in hour
three, repeated amplifier shutdowns during the second night, approximately
1h45 without a QSO, and heavy rain and thunderstorms Sunday.

**Test:** identify each activity gap in the log and RBN record, estimate a
range of score foregone at the surrounding rate, and compare matched-skimmer
reception before and after. Weather or lightning data is needed to corroborate
the QRN explanation; RBN SNR alone cannot do that.

## Initial log corroboration

The locked Stage 1 workflow already corroborates the timing of two reported
interruptions. It does not establish their causes.

- EF8R has no logged QSO in four consecutive five-minute buckets from
  12:20--12:40 UTC Sunday. CQ9A logged 54 QSOs and 157 QSO points during those
  20 minutes, reducing its reconstructed deficit from 2,629,095 to 2,464,671
  points: 164,424 points recovered, but no change of leader.
- 5J1DX has no logged QSO in 24 consecutive five-minute buckets from
  09:05--11:05 UTC Sunday. This is 15 minutes longer than the approximate 1h45
  reported on 3830, a difference compatible with rounding or the boundaries
  used by each account. CQ9A logged 473 QSOs and 1,389 QSO points in the same
  interval; its reconstructed advantage over 5J1DX widened by about 850,000.
- CQ9A has no comparable 15-minute-or-longer no-QSO interval. Its keyboard
  problem therefore requires finer rate, interleaving, and RBN analysis rather
  than a simple outage test.

RBN sharpens the interruption evidence. In EF8R's nominal 20-minute log gap,
28 of 30 spots occur during 12:20--12:22, followed by only isolated decodes at
12:29 and 12:32; broad decoding resumes at 12:40:36 on a new 10-meter
frequency. That is consistent with roughly 18 minutes away from the radios.
For 5J1DX, the entire two-hour no-QSO interval also contains zero exact-call
RBN spots. RBN therefore supports RF inactivity during both events, while the
operators' accounts remain the source for *why* they stopped.

The machine-readable event ledger is [`operator-events.csv`](operator-events.csv).

## Initial matched-skimmer evidence

The two official RBN daily archives contain 84,810 exact-call spots for the
three stations: 37,805 EF8R, 40,319 CQ9A, and 6,686 5J1DX. Raw totals are not a
signal-strength ranking because CQ frequency, operating style, and the uneven
skimmer network all affect spot counts.

`cqww-rbn-report` therefore compares EF8R and CQ9A only when the same skimmer
heard both on the same band within the same five-minute bucket. Each station's
spots are averaged within that receiver/bucket before taking the difference.
The initial weekend-wide results are:

| Band | Matched receiver/buckets | Mean EF8R minus CQ9A SNR |
| --- | ---: | ---: |
| 160m | 14 | -2.21 dB |
| 80m | 1,336 | -1.83 dB |
| 40m | 3,085 | -1.58 dB |
| 20m | 1,002 | +2.43 dB |
| 15m | 2,294 | -0.12 dB |
| 10m | 1,248 | -1.39 dB |

This **does not support a simple "EF8R was louder on the high bands"
explanation** for its decisive 10/15-meter QSO advantage. On 10 meters CQ9A
was 2.41 dB stronger at matched European receivers while EF8R was 0.85 dB
stronger at matched North American receivers. On 15 meters those differences
were 1.72 dB toward CQ9A in Europe and 2.05 dB toward EF8R in North America.

That geographic split is a derived observation, not yet a causal result. The
next test must align reach, CQ frequency occupancy, and log production by time
and region. Five-minute matching controls receiver identity, but not antenna
direction, the exact seconds between observations, or whether both stations
were CQing continuously.

### The two days were different contests

Joining `band-activity.csv` to the continent-level RBN reduction produces the
following high-band split. A skimmer-bucket is one distinct receiver hearing a
station in one five-minute interval, summed across 10 and 15 meters.

| Day | Station | 10/15m QSOs | Active 5m intervals | Both high bands seen | EU skimmer-buckets | NA skimmer-buckets |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Saturday | EF8R | 4,225 | 164 | 112 | 4,253 | 2,130 |
| Saturday | CQ9A | 3,696 | 166 | 126 | 4,743 | 2,435 |
| Sunday | EF8R | 2,579 | 185 | 95 | 6,051 | 3,652 |
| Sunday | CQ9A | 1,552 | 168 | 33 | 2,603 | 1,453 |

On Saturday, EF8R produced 529 more high-band QSOs despite CQ9A having more
RBN receiver/buckets and slightly more active intervals. This is evidence
against attributing the first-day advantage to broader raw reach. On Sunday,
EF8R added another 1,027-QSO advantage while appearing in 2.32 times as many
European and 2.51 times as many North American skimmer-buckets. It was heard
on both high bands within the same interval 95 times, versus only 33 for CQ9A.

Thus 68.2 percent of EF8R's final 1,506-QSO high-band advantage accumulated on
Sunday. The RBN evidence is consistent with much more sustained dual-high-band
CQ presence that day. It does not by itself distinguish propagation, antenna
choice, operator execution, or the effect of CQ9A's reported synchronization
problem.

## The online scoreboard was a spectator view, not telemetry

The archived [scoreboard rate tool][scoreboard-rate] retains hourly charts,
but reporting cadence creates artifacts. Its first-day EF8R/CQ9A chart has no
values for hours 13 and 14, followed by apparent hour-15 QSO totals of 1,187
and 982. The Cabrillo-derived hour-15 values are 351 and 330. The scoreboard
values are consistent with several delayed uploads being assigned to one
display interval, not either operator suddenly tripling the humanly plausible
rate.

Consequently, the scoreboard reliably documents the public ranking and final
reported state, but its hourly bars cannot be substituted for QSO timestamps.
This materially nuances the official article's description of watching the
race unfold: spectators saw a compelling race, but not a uniformly sampled
instrument trace.

## Comparison matrix

| Question | Official article | 3830 accounts | Stage 1 logs | Stage 2 test |
| --- | --- | --- | --- | --- |
| Who won? | EF8R | EF8R claimed lead | EF8R raw lead | Confirmed |
| Was it close throughout? | Calls it a 48-hour battle | Operators describe a showdown | Last lead change at 06:05; 1.49M gap by halfway | Compare live-score history with reconstructed lead |
| What decided EF8R-CQ9A? | Not stated | Operational context only | 10/15m QSO-point production | Add hourly geography and matched-skimmer RBN reach |
| Did interruptions matter? | Not stated | All three report problems | Timing partly visible | Quantify each interval and counterfactual range |
| How did 5J1DX fall away? | Only third-place total | Station and weather problems | High bands close to CQ9A; deficit elsewhere | Test outages plus regional/band reach |
| Was the spectator view accurate? | Not discussed | Multiple comments cite live score | Final archived value only | Acquire time-series snapshots if available |

## Guardrails

- A Cabrillo gap is not proof that a transmitter was off.
- An RBN gap is not proof of failure unless broad, previously active skimmer
  coverage disappears.
- RBN spot counts are biased; matched skimmers and short windows are preferred.
- A counterfactual points-lost estimate must be a range based on adjacent
  production, never a recovered official score.
- The official result remains authoritative for placing and records.

## Next reproducible outputs

1. five-minute QSO and score deltas around each reported event;
2. align RBN reach with log production and reported events;
3. live-score versus reconstructed score, if snapshots can be acquired; and
4. a claim ledger classifying statements as confirmed, nuanced, untestable,
   contradicted, or newly derived.

[official]: https://cqww.com/results/2025_cq_ww_dx_cw_results.pdf
[3830-summary]: https://3830scores.com/editionscores.php?arg=RvxuizV777Y0U
[3830-ef8r]: https://3830scores.com/showrumor.php?arg=RvYizV7nJx07uU
[3830-cq9a]: https://3830scores.com/showrumor.php?arg=RvYizV7nJxJx0U
[3830-5j1dx]: https://3830scores.com/showrumor.php?arg=RvYizV7nJx07eU
[scoreboard-rate]: https://contestonlinescore.com/tools/rate/?arc_contest_id=75&call=EF8R&call2=CQ9A
