# The Battle by the Logs

## How EF8R pulled away from CQ9A in CQ WW CW 2025

For roughly six hours, the world Single Operator High Power race in the 2025
CQ World Wide DX CW Contest was genuinely unsettled. Dan Craig, N6MJ, at EF8R
in the Canary Islands and Chris Hurlbut, KL9A, at CQ9A in Madeira traded the
lead six times in a uniform five-minute reconstruction. At 12 hours only
98,516 points separated them. By the end, EF8R's raw-log advantage was about
three million points.

The logs show that this was not principally a multiplier victory. It was a
high-band production victory.

Emir "Braco" Memic, E77DX, operating 5J1DX from Colombia, was the third member
of the public race. His high-band QSO volume was surprisingly close to CQ9A's.
Most of the separation between them accumulated on 20, 40, 80, and 160 meters.

## What was reconstructed

RadioSport Data Lab downloads the three public Cabrillo logs and the CTY-3537
country-file release dated November 25, 2025. It independently applies the
published CQ WW rules after every logged QSO:

- a station can be contacted once per band;
- intercontinental contacts earn three points;
- same-continent, different-country contacts earn one point, except that
  contacts between North American countries earn two;
- same-country contacts earn zero points but may add multipliers;
- countries and CQ zones count separately on each band; and
- maritime-mobile contacts count only as zone multipliers.

The result is sampled into aligned five-minute buckets. The claimed score is
never used to change the reconstruction. Sources and SHA-256 hashes are locked
in `source-lock.json` and verified on every workflow run.

The official rules define score as QSO points multiplied by countries plus
zones. They also establish the QSO-point and maritime-mobile rules used here.
[Official 2025 CQ WW rules](https://cqww.com/rules/2025_rules_cqww.pdf)

## The race at four checkpoints

The following table uses one uniform CTY-3537 interpretation for all three
logs. This makes station-to-station timing comparable even though the logging
programs evidently did not use identical multiplier maps.

| Hour | Rank | Station | Unique QSOs | QSO points | Countries | Zones | Score | Behind |
| ---: | ---: | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 12 | 1 | EF8R | 3,878 | 11,598 | 327 | 95 | 4,894,356 | - |
| 12 | 2 | CQ9A | 3,583 | 10,705 | 346 | 102 | 4,795,840 | 98,516 |
| 12 | 3 | 5J1DX | 2,740 | 8,146 | 279 | 91 | 3,014,020 | 1,880,336 |
| 24 | 1 | EF8R | 7,622 | 22,797 | 445 | 138 | 13,290,651 | - |
| 24 | 2 | CQ9A | 6,960 | 20,820 | 434 | 133 | 11,804,940 | 1,485,711 |
| 24 | 3 | 5J1DX | 5,787 | 17,186 | 385 | 131 | 8,867,976 | 4,422,675 |
| 36 | 1 | EF8R | 10,028 | 29,984 | 511 | 153 | 19,909,376 | - |
| 36 | 2 | CQ9A | 9,074 | 27,138 | 496 | 143 | 17,341,182 | 2,568,194 |
| 36 | 3 | 5J1DX | 7,075 | 21,002 | 464 | 151 | 12,916,230 | 6,993,146 |
| 48 | 1 | EF8R | 12,708 | 37,993 | 561 | 168 | 27,696,897 | - |
| 48 | 2 | CQ9A | 11,340 | 33,911 | 559 | 167 | 24,619,386 | 3,077,511 |
| 48 | 3 | 5J1DX | 9,218 | 27,310 | 501 | 161 | 18,079,220 | 9,617,677 |

### Finding 1: the first six hours were a real contest

**Classification: derived.** EF8R led the first five-minute bucket. The lead
then changed hands at approximately 00:50, 01:45, 03:05, 04:25, 04:55, and
06:05 UTC. EF8R took the lead at 06:05 and did not relinquish it in any later
five-minute bucket.

These are bucket-level changes, not second-accurate passings. Cabrillo records
times only to the minute, and cumulative score depends on the country map.

### Finding 2: CQ9A led the multiplier race at hour 12

**Classification: derived.** At 12 hours CQ9A had 448 multipliers to EF8R's
422, an advantage of 26. EF8R nevertheless led because it had accumulated 893
more QSO points. The point advantage was already large enough to overcome
CQ9A's 6.2 percent multiplier advantage.

During the next 12 hours EF8R added 161 multipliers while CQ9A added 119. At
the halfway point the multiplier positions had reversed: EF8R led by 16 and
the score margin had grown to 1.49 million.

### Finding 3: 10 and 15 meters decided the EF8R-CQ9A comparison

**Classification: derived.** The final band totals make the source of EF8R's
production advantage unusually clear.

| Band | EF8R unique QSOs | CQ9A unique QSOs | EF8R minus CQ9A | Point difference |
| --- | ---: | ---: | ---: | ---: |
| 10m | 3,292 | 2,502 | +790 | +2,367 |
| 15m | 3,366 | 2,650 | +716 | +2,136 |
| 20m | 2,289 | 2,293 | -4 | -13 |
| 40m | 2,274 | 2,130 | +144 | +430 |
| 80m | 1,195 | 1,389 | -194 | -586 |
| 160m | 292 | 376 | -84 | -252 |

Across 10 and 15 meters, EF8R made 1,506 more unique contacts and earned 4,503
more points. CQ9A recovered 278 contacts and 838 points on 80 and 160 meters,
but that was not enough. Their 20-meter totals were essentially identical.

In the uniform reconstruction EF8R finished with only three more multipliers.
Of the 3,077,511-point margin, 2,975,778 points, or 96.7 percent, are explained
by EF8R's 4,082-point QSO-point advantage at EF8R's multiplier level. The
remaining 101,733 points come from its three-multiplier advantage.

This decomposition is arithmetic, not a claim that each component could have
been changed independently.

### Finding 4: 5J1DX was competitive with CQ9A on the high bands

**Classification: derived.** 5J1DX recorded 2,507 unique contacts on 10 meters,
five more than CQ9A, and 2,715 on 15 meters, 65 more than CQ9A. Its entire net
QSO deficit to CQ9A arose elsewhere:

- 703 fewer contacts on 20 meters;
- 437 fewer on 40 meters;
- 826 fewer on 80 meters; and
- 226 fewer on 160 meters.

The four-band deficit was 2,192 contacts, partially offset by the 70-contact
advantage on 10 and 15. This does not establish why those differences occurred.
Station readiness, propagation, geography, and operating choices are candidate
explanations for later stages of analysis, not conclusions from the logs alone.

### Finding 5: the raw score has more than one legitimate representation

**Classification: derived and confirmed against public sources.** EF8R and
5J1DX reproduce the QSO-point totals implied by their Cabrillo claimed scores.
CQ9A reproduces the point total implied by its archived online score. The
multiplier counts do not all match the uniform CTY-3537 interpretation:

| Station | Uniform points | Uniform mults | Uniform score | Cabrillo claimed score | Header arithmetic |
| --- | ---: | ---: | ---: | ---: | --- |
| EF8R | 37,993 | 729 | 27,696,897 | 27,582,918 | 37,993 x 726 |
| CQ9A | 33,911 | 726 | 24,619,386 | 24,689,392 | 33,914 x 728 |
| 5J1DX | 27,310 | 662 | 18,079,220 | 18,024,600 | 27,310 x 660 |

The archived online score for CQ9A is 24,687,208. The subsequently published
Cabrillo header says 24,689,392, exactly three QSO points times 728 multipliers
higher. The raw QSO content still reconstructs to 33,911 points. This is
consistent with a one-contact change without a refreshed claimed-score header,
but the public evidence does not establish the cause. [Archived online
scoreboard](https://contestonlinescore.com/archive/?arc_contest_id=75)

The safest conclusion is that each logging environment had its own multiplier
interpretation. CTY-3537 is used here as a common analytical ruler, not as a
claim about the precise country file installed at each station.

## Stage 2: what the RBN adds

The Reverse Beacon Network supplies independent receiver evidence: timestamped
decodes from many skimmers, grouped into the same five-minute intervals as the
logs. It cannot tell us operator intent or measure either station in a
laboratory sense, but it can test whether a logged interruption coincides with
an observable loss of RF and whether apparent reach differs by continent.

### Finding 6: Saturday and Sunday were different races

**Classification: derived from raw logs and RBN observations.** EF8R's final
1,506-contact advantage on 10 and 15 meters was heavily back-loaded: 529 of
those contacts came on Saturday and 1,027 on Sunday. Thus 68.2 percent of the
high-band QSO advantage accumulated on the second day.

| UTC day | Station | 10m + 15m QSOs | Active 5m intervals | Both high bands active | EU skimmer-buckets | NA skimmer-buckets |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Nov. 29 | EF8R | 4,225 | 164 | 112 | 4,253 | 2,130 |
| Nov. 29 | CQ9A | 3,696 | 166 | 126 | 4,743 | 2,435 |
| Nov. 30 | EF8R | 2,579 | 185 | 95 | 6,051 | 3,652 |
| Nov. 30 | CQ9A | 1,552 | 168 | 33 | 2,603 | 1,453 |

On Saturday CQ9A was decoded by more European and North American receivers and
was active in slightly more five-minute intervals, yet EF8R completed 529 more
high-band contacts. On Sunday EF8R combined a 1,027-contact production
advantage with much broader observed reach: 2.32 times CQ9A's European
skimmer-buckets and 2.51 times its North American total. EF8R also appeared on
both 10 and 15 meters in 95 intervals versus CQ9A's 33. These observations do
not isolate station design, propagation, or operating decisions, but they
locate the decisive separation much more precisely than the final score does.

### Finding 7: matched receivers complicate a simple signal-strength story

**Classification: derived.** Comparing only skimmers that decoded both stations
on the same band in the same five-minute interval reduces receiver and timing
bias. On the high bands the geographic split mattered: CQ9A led the median
matched comparison into Europe, while EF8R led into North America.

| Band | Europe: EF8R minus CQ9A | North America: EF8R minus CQ9A |
| --- | ---: | ---: |
| 10m | -2.41 dB | +0.85 dB |
| 15m | -1.72 dB | +2.05 dB |

The unexpected result is that EF8R's QSO advantage is not explained by being
uniformly louder everywhere. The evidence instead points toward a combination
of geographic reach, sustained availability, and conversion of openings into
contacts.

### Finding 8: the two major log gaps have independent RF corroboration

**Classification: corroborated, not causal.** EF8R logged no QSOs from 12:20
through 12:39 UTC Sunday. RBN has 30 exact-call spots in the nominal interval,
but 28 occur during its first two minutes; only isolated decodes appear at
12:29 and 12:32. Broad decoding resumes at 12:40:36 on a new 10-meter
frequency. This is consistent with roughly 18 minutes of RF cessation, while
not identifying its cause. CQ9A gained about 164,000 reconstructed points over
the same 20-minute interval.

5J1DX's two-hour log gap from 09:05 to 11:05 UTC Sunday is even cleaner: there
are no exact-call RBN spots during the entire interval. CQ9A made 473 contacts
and 1,389 QSO points during it, widening the reconstructed gap by roughly
850,000 points.

### Finding 9: the archived scoreboard is a spectator record, not telemetry

**Classification: publicly observed and compared with logs.** The archived
rate chart omits intermediate values around hours 13 and 14 and then displays
an apparent hour-15 surge of about 1,187 QSOs for EF8R and 982 for CQ9A. Their
Cabrillo logs contain 351 and 330 contacts in that hour. The mismatch is a
clear delayed-upload batching artifact. The scoreboard preserves what viewers
saw, but its hour-to-hour shape should not be used as a station-rate trace.
[Archived rate comparison](https://contestonlinescore.com/tools/rate/?arc_contest_id=75&call=EF8R&call2=CQ9A)

## Checked results: context, not reconstructed adjudication

The official results were EF8R 26,516,025, CQ9A 23,779,792, and 5J1DX
16,738,525. EF8R won the category and established the published world record;
CQ9A was second and 5J1DX third. [Official 2025 CQ WW CW
results](https://cqww.com/results/2025_cq_ww_dx_cw_results.pdf)

This Stage 1 analysis does not infer why individual contacts were removed or
penalized. The public Cabrillo logs are raw submissions, while the necessary
contact-level adjudication evidence is not public here. Checked totals are
therefore reported only as aggregate context.

## Limitations

- A logged time is minute-resolution evidence of a completed contact, not a
  continuous record of both radios or operator intent.
- The timeline uses a uniform historical country file; station loggers used
  different or locally modified multiplier data.
- Duplicate means another occurrence of the same call on the same band. It
  does not by itself imply carelessness; repeat contacts are often logged when
  an operator is uncertain of a previous QSO.
- Lead changes are reconstructed at five-minute resolution and may move
  slightly under another valid multiplier map.
- RBN spot counts depend on the participating receiver population. A
  skimmer-bucket is one distinct receiver in one five-minute interval, not a
  calibrated coverage-area measurement.
- Matched-receiver SNR reduces important biases but does not eliminate antenna,
  path, frequency, fading, or transmit-cycle differences.
- The archived scoreboard has delayed-upload artifacts and is used only as a
  record of the public viewing experience.

## Reproduce it

From the RadioSport Data Lab repository:

```bash
./scripts/run-cqww-cw-2025-battle.sh
```

The workflow verifies every downloaded input against the committed lock and
writes `summaries.csv`, `score-timeline.csv`, `band-activity.csv`,
`lead-changes.csv`, `checkpoints.csv`, and `three-hour-checkpoints.csv`. The RBN
workflow adds reach, matched-receiver, daily high-band, and QuantaStream JSONL
outputs. See the case-study `README.md` for
paths and environment overrides.

The point of the exercise is not that an AI can summarize a scoreboard. Codex
helped turn public evidence into testable questions and trace discrepancies;
RadioSport Data Lab made the workflow repeatable; and QuantaStream provides the
analytical foundation for querying the same evidence model at larger scale.
