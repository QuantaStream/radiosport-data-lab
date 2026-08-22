# RBN Archive Profile: 2026-08-21

Downloaded sample:

```text
https://data.reversebeacon.net/rbn_history/20260821.zip
```

Archive contents:

```text
20260821.csv
```

The CSV ends with a footer line:

```text
(278552 rows)
```

The parser skips that footer and treats the file as 278,552 data rows.

## Header

```text
callsign,de_pfx,de_cont,freq,band,dx,dx_pfx,dx_cont,mode,db,date,speed,tx_mode
```

## Observed Profile

| Field | Observation |
| --- | --- |
| `callsign` | 219 unique spotter calls in the sample; length 3..13. |
| `dx` | 7,160 unique DX calls in the sample; length 3..12. |
| `de_pfx` | 48 unique prefix values. |
| `dx_pfx` | 183 unique prefix values. |
| `de_cont` / `dx_cont` | 6 observed continent values each. |
| `freq` | 1747.9..144089.9 kHz. |
| `band` | 13 observed band values. |
| `mode` | `CQ`, `NCDXF B`, `BEACON`, `DX`. |
| `db` | -2..94. |
| `speed` | 0..80 WPM. |
| `tx_mode` | `CW`, `RTTY`. |

Top observed bands:

```text
20m=134445
40m=62137
15m=20056
30m=18980
17m=18730
10m=9472
80m=7421
6m=3089
12m=2356
160m=1077
60m=637
4m=123
2m=29
```

Top observed spotter prefixes:

```text
K=75238
DL=45264
G=15879
S5=13381
LZ=11385
ES=8008
OE=7067
PA=6870
SM=6690
GM=6428
```

Top observed DX prefixes:

```text
K=71650
DL=28511
I=19328
UA=13520
F=12377
G=11994
EA=11220
SP=6517
LZ=5899
JA=5775
```
