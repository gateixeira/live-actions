# Load Testing

This directory contains load tests for the live-actions service using [k6](https://k6.io/).

## Webhook Load Test

The `webhook-load.js` script simulates realistic GitHub Actions webhook traffic by sending `workflow_job` and `workflow_run` events through the full lifecycle (queued → in_progress → completed).

### Test Scenario

The test simulates 15 different label combinations across three runner types:

- **GitHub-hosted** (5 labels): `ubuntu-latest`, `windows-latest`, `macos-latest`, etc.
- **Self-hosted single** (5 labels): `self-hosted`, `linux`, `gpu`, etc.
- **Combined label arrays** (5 sets): `[self-hosted, linux, x64]`, `[self-hosted, gpu, cuda]`, etc.

Each virtual user runs a full job lifecycle per iteration:

1. Sends `queued` event (job + optionally a workflow run, 60% chance)
2. Waits a random queue time (weighted: 30% fast, 40% normal, 20% slow, 10% very slow)
3. Sends `in_progress` event
4. Waits 2–5 seconds (simulated execution)
5. Sends `completed` event with random outcome (success/failure/cancelled)

### Running the Test

1. Install [k6](https://k6.io/docs/get-started/installation/)
2. Set `WEBHOOK_SECRET` in the script to match your server configuration
3. Run:

```bash
k6 run webhook-load.js
```

### Configuration

Modify the `options` object in the script to adjust load profile:

```js
stages: [
  { duration: "30s", target: 15 },   // Ramp up to 15 VUs
  { duration: "200s", target: 15 },  // Hold at 15 VUs
  { duration: "20s", target: 0 },    // Ramp down
]
```

Or override via CLI:

```bash
k6 run --vus 20 --duration 2m webhook-load.js
```
