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
- No RBN signal, propagation, audio, or station-performance inference is part
  of Stage 1.
- Online-score history is not required. Only its archived end value is used to
  document the CQ9A raw-score inconsistency.

## Reproduce it

From the RadioSport Data Lab repository:

```bash
./scripts/run-cqww-cw-2025-battle.sh
```

The workflow verifies every downloaded input against the committed lock and
writes `summaries.csv`, `score-timeline.csv`, `band-activity.csv`,
`lead-changes.csv`, and `checkpoints.csv`. See the case-study `README.md` for
paths and environment overrides.

The point of the exercise is not that an AI can summarize a scoreboard. Codex
helped turn public evidence into testable questions and trace discrepancies;
RadioSport Data Lab made the workflow repeatable; and QuantaStream provides the
analytical foundation for extending the same model to millions of RBN
observations in the next stage.
