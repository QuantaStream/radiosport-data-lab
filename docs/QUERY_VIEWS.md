# Query Views

This repository keeps reusable analyst-facing views under `sql/views`.

## `rbn_spot_propagation_base`

`rbn_spot_propagation_base` is the first general-purpose view for Workbench,
Tableau, and ad hoc SQL. It starts with `spots_flat`, then joins in daily SWPC
solar indices and three-hour K/Kp buckets through relationship-vector fields.

Create or refresh the view:

```bash
mysql -h 127.0.0.1 -P 4000 -u qstream -D quanta \
  < sql/views/rbn_spot_propagation_base.sql
```

The view intentionally avoids QRZ profile data. QRZ enrichment is sparse and is
better queried through focused joins from `spots` until QS has broader left join
coverage for optional relationships.

Useful smoke queries:

```sql
select count(*) as spots
from rbn_spot_propagation_base;
```

```sql
select
  band,
  dx_prefix,
  sfi,
  kp_index,
  count(*) as spots
from rbn_spot_propagation_base
where spotted_at between todate('2025-11-29') and todate('2025-12-01')
group by band, dx_prefix, sfi, kp_index
order by spots desc
limit 50;
```

```sql
select
  band,
  dx_prefix,
  sfi,
  kp_index,
  count(*) as spots
from rbn_spot_propagation_base
where spotted_at between todate('2025-11-29') and todate('2025-12-01')
  and dx_call = 'TI8X'
group by band, dx_prefix, sfi, kp_index
order by spots desc
limit 50;
```

```sql
select
  spotter_continent,
  dx_continent,
  band,
  count(*) as spots
from rbn_spot_propagation_base
where spotted_at between todate('2025-11-29') and todate('2025-12-01')
group by spotter_continent, dx_continent, band
order by spots desc
limit 50;
```
