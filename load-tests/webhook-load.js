// k6 load test for live-actions POST /webhook.
//
// Purpose: benchmark the webhook ingest path (HMAC validation +
// EventOrderingService.AddEvent) at a defined arrival rate so results are
// directly comparable across two builds (baseline vs improved).
//
// Two scenarios, selected via SCENARIO env var:
//   - stress    (default): ramping arrival rate to find the breaking point.
//   - sustained: fixed arrival rate to compare steady-state latency/throughput.
//
// Why constant-arrival-rate (and not VU-based loops)?
//   A VU-based loop throttles when the server is slow, which masks the
//   ingest bottleneck we're trying to measure. With constant-arrival-rate,
//   k6 enforces a request rate independent of server response time, so
//   queueing behavior, p95/p99 latency, and 5xx rate become honest signals.
//
// Required env vars:
//   WEBHOOK_SECRET    Same value as the server's WEBHOOK_SECRET.
//
// Optional env vars:
//   BASE_URL          Default: http://localhost:8080
//   SCENARIO          stress | sustained                 (default: stress)
//   RATE              Sustained: events/sec              (default: 200)
//   DURATION          Sustained: hold duration           (default: 2m)
//   INFINITE          Sustained: run until Ctrl+C        (default: false)
//   PEAK_RATE         Stress:    peak events/sec         (default: 2000)
//   PREALLOC_VUS      Stress:    initial VU pool         (default: 500)
//   MAX_VUS           Stress:    upper bound on VU pool  (default: 4000)
//   RESULT_TAG        Label written into the summary     (default: "run")
//
// Examples:
//   k6 run -e WEBHOOK_SECRET=$WEBHOOK_SECRET -e RESULT_TAG=baseline \
//          load-tests/webhook-load.js
//
//   k6 run -e WEBHOOK_SECRET=$WEBHOOK_SECRET -e SCENARIO=sustained \
//          -e RATE=500 -e DURATION=3m -e RESULT_TAG=improved-500rps \
//          load-tests/webhook-load.js

import http from "k6/http";
import crypto from "k6/crypto";
import { check } from "k6";
import { Counter, Trend, Rate } from "k6/metrics";
import { randomIntBetween, randomItem } from "https://jslib.k6.io/k6-utils/1.4.0/index.js";

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

const WEBHOOK_SECRET = __ENV.WEBHOOK_SECRET;
if (!WEBHOOK_SECRET) {
  throw new Error("WEBHOOK_SECRET env var is required");
}

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";
const SCENARIO = (__ENV.SCENARIO || "stress").toLowerCase();
const RESULT_TAG = __ENV.RESULT_TAG || "run";

const SUSTAINED_RATE = parseInt(__ENV.RATE || "200", 10);
// INFINITE=true forces a 30-day duration so the test runs until you Ctrl+C.
// k6 has no true "infinite" mode; this is the idiomatic workaround. Partial
// summaries are still written by handleSummary on interrupt.
const INFINITE = String(__ENV.INFINITE || "").toLowerCase() === "true";
const SUSTAINED_DURATION = INFINITE ? "720h" : (__ENV.DURATION || "2m");
const STRESS_PEAK_RATE = parseInt(__ENV.PEAK_RATE || "2000", 10);

// ---------------------------------------------------------------------------
// Custom metrics — make the comparison-relevant signals first-class.
// ---------------------------------------------------------------------------

const webhookLatency = new Trend("webhook_latency_ms", true);
const accepted202 = new Counter("webhook_accepted_202");
const queueFull503 = new Counter("webhook_queue_full_503");
const otherErrors = new Counter("webhook_other_errors");
const errorRate = new Rate("webhook_error_rate");
const eventsByType = new Counter("webhook_events_by_type");

// ---------------------------------------------------------------------------
// Scenario / threshold setup
// ---------------------------------------------------------------------------

const scenarios = {
  stress: {
    // Ramping arrival rate: walks the request rate up so we can spot the
    // exact RPS where latency tail / errors break out. preAllocatedVUs
    // must be large enough that the executor never starves under peak load.
    executor: "ramping-arrival-rate",
    startRate: 50,
    timeUnit: "1s",
    preAllocatedVUs: parseInt(__ENV.PREALLOC_VUS || "500", 10),
    maxVUs: parseInt(__ENV.MAX_VUS || "4000", 10),
    stages: [
      { duration: "30s", target: 100 },
      { duration: "30s", target: 250 },
      { duration: "30s", target: 500 },
      { duration: "30s", target: 1000 },
      { duration: "60s", target: STRESS_PEAK_RATE },
      { duration: "20s", target: 0 },
    ],
  },
  sustained: {
    // Fixed rate for a steady-state, apples-to-apples comparison.
    executor: "constant-arrival-rate",
    rate: SUSTAINED_RATE,
    timeUnit: "1s",
    duration: SUSTAINED_DURATION,
    preAllocatedVUs: Math.max(50, Math.ceil(SUSTAINED_RATE / 4)),
    maxVUs: Math.max(200, SUSTAINED_RATE * 2),
  },
};

if (!scenarios[SCENARIO]) {
  throw new Error(`Unknown SCENARIO: ${SCENARIO} (expected stress|sustained)`);
}

export const options = {
  scenarios: { [SCENARIO]: scenarios[SCENARIO] },
  // Thresholds intentionally permissive: we want results, not red runs.
  // Treat them as "did the server stay up?" sanity checks, not pass/fail.
  thresholds: {
    webhook_error_rate: ["rate<0.10"],
    webhook_latency_ms: ["p(95)<2000", "p(99)<5000"],
  },
  // Compress stdout summary so it's easy to copy-paste into a comparison.
  summaryTrendStats: ["avg", "min", "med", "p(90)", "p(95)", "p(99)", "max"],
  discardResponseBodies: true,
};

// ---------------------------------------------------------------------------
// Payload generation
// ---------------------------------------------------------------------------

const RUNNER_LABELS = [
  ["ubuntu-latest"],
  ["windows-latest"],
  ["macos-latest"],
  ["ubuntu-22.04"],
  ["self-hosted"],
  ["self-hosted", "linux", "x64"],
  ["self-hosted", "gpu", "cuda"],
  ["self-hosted", "macos", "arm64"],
];

const REPOS = [
  { name: "demo-app", url: "https://github.com/example/demo-app" },
  { name: "infra", url: "https://github.com/example/infra" },
  { name: "frontend", url: "https://github.com/example/frontend" },
];

// Status mix mirrors the rough distribution seen by a busy GH org: lots of
// queued/in_progress activity, fewer terminal events.
const JOB_ACTIONS = [
  { action: "queued", weight: 50 },
  { action: "in_progress", weight: 30 },
  { action: "completed", weight: 20 },
];

const RUN_ACTIONS = [
  { action: "requested", weight: 40 },
  { action: "in_progress", weight: 35 },
  { action: "completed", weight: 25 },
];

function weightedPick(items) {
  const total = items.reduce((s, i) => s + i.weight, 0);
  let r = Math.random() * total;
  for (const i of items) {
    if ((r -= i.weight) <= 0) return i.action;
  }
  return items[items.length - 1].action;
}

// Build a unique-ish ID that fits in both int64 and JS's safe-integer range
// (2^53 - 1 = 9_007_199_254_740_991). Numbers above that are serialized in
// scientific notation by JSON.stringify and Go's int64 unmarshal rejects them.
// Layout: VU * 1e13 + (iter % 1e7) * 1e6 + random(0, 999_999)
// With up to 900 VUs this stays under ~9e15.
function uniqueID() {
  const vuPart = (__VU % 900) * 1e13;
  const iterPart = (__ITER % 1e7) * 1e6;
  const randPart = randomIntBetween(0, 999999);
  return vuPart + iterPart + randPart;
}

function nowISO(offsetSec = 0) {
  return new Date(Date.now() + offsetSec * 1000).toISOString();
}

function buildWorkflowJobPayload() {
  const action = weightedPick(JOB_ACTIONS);
  const id = uniqueID();
  const runID = uniqueID();
  const labels = randomItem(RUNNER_LABELS);
  const createdAt = nowISO(-randomIntBetween(0, 60));

  const job = {
    id,
    run_id: runID,
    name: `job-${id}`,
    status: action,
    labels,
    html_url: `https://github.com/example/demo-app/actions/runs/${runID}/job/${id}`,
    conclusion: action === "completed" ? randomItem(["success", "failure", "cancelled"]) : "",
    created_at: createdAt,
    started_at: action === "queued" ? "0001-01-01T00:00:00Z" : nowISO(-randomIntBetween(0, 30)),
    completed_at: action === "completed" ? nowISO() : "0001-01-01T00:00:00Z",
  };

  return { action, workflow_job: job };
}

function buildWorkflowRunPayload() {
  const action = weightedPick(RUN_ACTIONS);
  const id = uniqueID();
  const repo = randomItem(REPOS);
  const createdAt = nowISO(-randomIntBetween(0, 60));

  const run = {
    id,
    name: `run-${id}`,
    status: action,
    html_url: `${repo.url}/actions/runs/${id}`,
    display_title: `chore: load-test ${id}`,
    conclusion: action === "completed" ? randomItem(["success", "failure"]) : "",
    created_at: createdAt,
    run_started_at: action === "requested" ? "0001-01-01T00:00:00Z" : nowISO(-randomIntBetween(0, 30)),
    updated_at: nowISO(),
  };

  return { action, repository: repo, workflow_run: run };
}

// ---------------------------------------------------------------------------
// Request execution
// ---------------------------------------------------------------------------

function sign(body) {
  return "sha256=" + crypto.hmac("sha256", WEBHOOK_SECRET, body, "hex");
}

function deliveryID() {
  // Mimic GitHub's delivery UUIDs closely enough for any logging that keys on shape.
  const hex = (n) => randomIntBetween(0, 0xffffffff).toString(16).padStart(n, "0");
  return `${hex(8)}-${hex(4).slice(0, 4)}-${hex(4).slice(0, 4)}-${hex(4).slice(0, 4)}-${hex(8)}${hex(4).slice(0, 4)}`;
}

export default function () {
  const isJob = Math.random() < 0.8;
  const eventType = isJob ? "workflow_job" : "workflow_run";
  const payload = isJob ? buildWorkflowJobPayload() : buildWorkflowRunPayload();
  const body = JSON.stringify(payload);

  const headers = {
    "Content-Type": "application/json",
    "X-GitHub-Event": eventType,
    "X-GitHub-Delivery": deliveryID(),
    "X-Hub-Signature-256": sign(body),
    "User-Agent": "GitHub-Hookshot/k6-loadtest",
  };

  const res = http.post(`${BASE_URL}/webhook`, body, { headers, tags: { event_type: eventType } });

  webhookLatency.add(res.timings.duration, { event_type: eventType });
  eventsByType.add(1, { event_type: eventType, action: payload.action });

  // 202 is the success path. 503 means the queue-full back-pressure path
  // engaged (only meaningful on the improved build). Anything else is a bug.
  if (res.status === 202) {
    accepted202.add(1);
    errorRate.add(false);
  } else if (res.status === 503) {
    queueFull503.add(1);
    errorRate.add(true);
  } else {
    otherErrors.add(1, { status: String(res.status) });
    errorRate.add(true);
  }

  check(res, {
    "status is 202 or 503": (r) => r.status === 202 || r.status === 503,
  });
}

// ---------------------------------------------------------------------------
// Result reporting
// ---------------------------------------------------------------------------

export function handleSummary(data) {
  const ts = new Date().toISOString().replace(/[:.]/g, "-");
  const filename = `load-tests/results/${RESULT_TAG}-${SCENARIO}-${ts}`;
  const compact = compactSummary(data);

  return {
    stdout: textSummary(compact),
    [`${filename}.json`]: JSON.stringify({ tag: RESULT_TAG, scenario: SCENARIO, ...compact }, null, 2),
    [`${filename}.txt`]: textSummary(compact),
  };
}

function compactSummary(data) {
  const m = data.metrics;
  const pick = (name, fields = ["avg", "med", "p(90)", "p(95)", "p(99)", "max"]) => {
    if (!m[name] || !m[name].values) return null;
    const out = {};
    for (const f of fields) {
      if (m[name].values[f] !== undefined) out[f] = m[name].values[f];
    }
    return out;
  };
  const counter = (name) => (m[name] && m[name].values ? m[name].values.count || 0 : 0);

  const totalRequests = (m.http_reqs && m.http_reqs.values && m.http_reqs.values.count) || 0;
  const testDurationSec = data.state.testRunDurationMs / 1000;

  return {
    test_duration_seconds: round(testDurationSec, 2),
    total_requests: totalRequests,
    requests_per_second: round(totalRequests / Math.max(testDurationSec, 0.001), 2),
    accepted_202: counter("webhook_accepted_202"),
    queue_full_503: counter("webhook_queue_full_503"),
    other_errors: counter("webhook_other_errors"),
    error_rate: m.webhook_error_rate && m.webhook_error_rate.values && m.webhook_error_rate.values.rate,
    webhook_latency_ms: pick("webhook_latency_ms"),
    http_req_duration_ms: pick("http_req_duration"),
    http_req_blocked_ms: pick("http_req_blocked", ["avg", "p(95)", "max"]),
    http_req_waiting_ms: pick("http_req_waiting", ["avg", "p(95)", "p(99)", "max"]),
    iterations: counter("iterations"),
  };
}

function round(n, p) {
  const f = Math.pow(10, p);
  return Math.round(n * f) / f;
}

function textSummary(s) {
  const fmt = (n) => (typeof n === "number" ? n.toFixed(2) : "n/a");
  const lat = s.webhook_latency_ms || {};
  return [
    `\n========== ${RESULT_TAG} / ${SCENARIO} ==========`,
    `duration:           ${fmt(s.test_duration_seconds)} s`,
    `total_requests:     ${s.total_requests}`,
    `requests_per_sec:   ${fmt(s.requests_per_second)}`,
    ``,
    `accepted (202):     ${s.accepted_202}`,
    `queue full (503):   ${s.queue_full_503}`,
    `other errors:       ${s.other_errors}`,
    `error_rate:         ${fmt((s.error_rate || 0) * 100)} %`,
    ``,
    `webhook_latency_ms  avg=${fmt(lat.avg)}  med=${fmt(lat["med"])}  p90=${fmt(lat["p(90)"])}  p95=${fmt(lat["p(95)"])}  p99=${fmt(lat["p(99)"])}  max=${fmt(lat.max)}`,
    `===================================\n`,
  ].join("\n");
}
