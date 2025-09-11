# Load Testing

This directory contains load tests for the live-actions service using [k6](https://k6.io/).

## Webhook Load Test

The `webhook-load.js` script simulates webhook events for the GitHub Actions runners demand API.

### Test Scenario

This test simulates three types of webhook events:

- `queued` jobs
- `in_progress` jobs
- `completed` jobs

Features:

- Randomly creates different job status types
- 50% chance for a job to have a "self-hosted" label, 50% chance to have empty labels
- During rampdown phase, all queued and in_progress jobs are completed before the test ends
- Tracks custom metrics for all job types

### Running the Test

1. Make sure you have k6 installed
2. Run the test:

```bash
# Using k6 directly
k6 run webhook-load.js
```

3. Customize the test parameters:

You can modify the stages in the options object to adjust:

- Duration of test
- Number of virtual users
- Ramp-up and ramp-down behavior

For example:

```bash
# Run with 20 VUs for 2 minutes
k6 run --vus 20 --duration 2m webhook-load.js
```

### Metrics

The test tracks the following custom metrics:

- `successful_queued_jobs`: Count of successfully sent queued job events
- `successful_in_progress_jobs`: Count of successfully sent in-progress job events
- `successful_completed_jobs`: Count of successfully sent completed job events
- `failed_requests`: Rate of failed requests
