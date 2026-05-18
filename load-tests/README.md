# Load Tests

k6 webhook ingest benchmark for `live-actions`. The point of these tests is to
produce **directly comparable numbers** between two builds of the server
(e.g. baseline `main` vs the SQLite/ingest changes on a feature branch).

All tests live in `webhook-load.js`. Results are written to `results/` per run
(see "Comparing results" below).

## Why this test is shaped the way it is

- **Constant-arrival-rate executor** (not VU loops). A VU loop slows down when
  the server slows down, so it would mask the very bottleneck we're trying to
  measure. With constant-arrival-rate, k6 enforces the request rate
  independently of server response time, so `p95`/`p99`/`5xx-rate` become
  honest signals of server health.
- **Fresh IDs per request.** Repeated job/run IDs would hit the UPSERT
  fast-path on existing rows and underestimate writer-pool pressure.
- **Realistic event mix:** 80% `workflow_job`, 20% `workflow_run`, statuses
  weighted toward `queued`/`in_progress` (terminal `completed` events are
  rarer in real life).
- **Two scenarios**, selected via `SCENARIO`:
  - `stress` *(default)* — ramping arrival rate to find the breaking point.
  - `sustained` — fixed rate for a steady-state, apples-to-apples comparison.

## Prerequisites

1. Install k6: <https://k6.io/docs/get-started/installation/>
2. Start the server with a known `WEBHOOK_SECRET` and (recommended)
   `LOG_LEVEL=warn` to avoid log I/O dominating the measurement:
   ```bash
   export WEBHOOK_SECRET=$(openssl rand -hex 32)
   LOG_LEVEL=warn make run
   ```
3. Export the same secret into the shell that runs k6:
   ```bash
   export WEBHOOK_SECRET=<same value>
   ```

## Recommended comparison workflow

Run the **same scenario twice**: once against the baseline build, once against
the improved build. Use `RESULT_TAG` to label the run; result files are written
to `results/<tag>-<scenario>-<timestamp>.{json,txt}`.

### 1. Find the breaking point (stress)

```bash
# Baseline
k6 run -e RESULT_TAG=baseline load-tests/webhook-load.js

# Switch branches / rebuild / restart server, then:
k6 run -e RESULT_TAG=improved load-tests/webhook-load.js
```

The `stress` scenario ramps from 50 → 2000 events/s. Look at where the
**latency tail breaks out** (p95/p99 spikes, queue-full 503s appear).

### 2. Steady-state comparison (sustained)

Pick a target rate just below the *baseline's* breaking point — that's where
the improvement should show up most clearly.

```bash
# Baseline
k6 run -e SCENARIO=sustained -e RATE=500 -e DURATION=3m \
       -e RESULT_TAG=baseline load-tests/webhook-load.js

# Improved
k6 run -e SCENARIO=sustained -e RATE=500 -e DURATION=3m \
       -e RESULT_TAG=improved load-tests/webhook-load.js
```

## What to read from the output

The custom text summary printed at the end of each run contains the only
numbers you need for a comparison:

```
========== improved / sustained ==========
duration:           180.42 s
total_requests:     90108
requests_per_sec:   499.45

accepted (202):     90108
queue full (503):   0
other errors:       0
error_rate:         0.00 %

webhook_latency_ms  avg=...  p95=...  p99=...  max=...
===================================
```

Pay attention to:
- **`requests_per_sec` actually achieved** vs the target rate — if the server
  can't keep up, k6 falls behind the schedule.
- **`p95` / `p99` latency** — the most sensitive signal of writer-pool
  contention.
- **`queue full (503)`** — only the improved build can return 503; on the
  baseline a saturated server shows up as latency growth + timeouts instead.
- **`other errors`** — should be 0 on both builds. Anything else is a real bug.

## Environment variables

| Var | Default | Notes |
|---|---|---|
| `WEBHOOK_SECRET` | *(required)* | Must match the server's secret |
| `BASE_URL` | `http://localhost:8080` | Point at the running server |
| `SCENARIO` | `stress` | `stress` or `sustained` |
| `RATE` | `200` | `sustained` only — events/sec |
| `DURATION` | `2m` | `sustained` only — hold duration |
| `PEAK_RATE` | `2000` | `stress` only — peak events/sec at top of the ramp |
| `RESULT_TAG` | `run` | Label written into result filenames + summary header |

## Comparing results

`results/*.json` has a stable, compact shape (one object per run) so you can
diff or aggregate them with any tool, e.g.:

```bash
jq '{tag, scenario, requests_per_second, error_rate, p95: .webhook_latency_ms["p(95)"], p99: .webhook_latency_ms["p(99)"]}' \
   load-tests/results/baseline-sustained-*.json \
   load-tests/results/improved-sustained-*.json
```

## Notes & caveats

- Run k6 **on a different machine from the server** if possible — co-location
  on a laptop will cap RPS at whatever the local CPU can do for both sides.
  When that isn't possible, prefer the *relative* comparison between the two
  runs over the absolute numbers.
- The default thresholds (`p95<2000ms`, `error_rate<10%`) are intentionally
  permissive. They exist to mark catastrophic regressions; don't read them as
  pass/fail criteria for the comparison itself.
- The server shares its DB with the production data dir by default. Point
  `DATABASE_PATH` at a throwaway file before stress runs:
  ```bash
  DATABASE_PATH=/tmp/live-actions-loadtest.db LOG_LEVEL=warn make run
  ```
