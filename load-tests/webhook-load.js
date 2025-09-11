// K6 load test script for GitHub webhook events
// Simulates both workflow_job and workflow_run events to test the webhook endpoints
// 
// This script tests 15 different label combinations:
// - 5 GitHub-hosted runner labels (ubuntu-latest, windows-latest, etc.)
// - 5 Self-hosted single labels (self-hosted, linux, gpu, etc.) 
// - 5 Combined label arrays (self-hosted + additional labels)
//
// The test simulates realistic GitHub Actions workflows with varied job types,
// queue times, and runner configurations to stress-test the label metrics system.
import http from 'k6/http';
import crypto from 'k6/crypto';
import { check, sleep } from "k6";

// Webhook secret
const secret = 'your_secret_here';

// Label distribution tracking
let labelStats = {};

// Helper function to track label usage
function trackLabelUsage(labels) {
  const labelKey = labels.sort().join(',');
  if (!labelStats[labelKey]) {
    labelStats[labelKey] = 0;
  }
  labelStats[labelKey]++;
}

// Helper function to generate signature
function generateSignature(payload) {
  const hmac = crypto.createHMAC('sha256', secret);
  hmac.update(payload);
  return `sha256=${hmac.digest('hex')}`;
}

// Test configuration
export const options = {
    thresholds: {
      http_req_duration: ["p(99) < 300"],
    },
    stages: [
      { duration: "30s", target: 15 },
      { duration: "200s", target: 15 },
      { duration: "20s", target: 0 },
    ],
};

// Summary function to display final statistics
export function handleSummary(data) {
  if (__VU === 1) {
    let totalJobs = 0;
    Object.values(labelStats).forEach(count => totalJobs += count);

    // Group by runner type for better readability
    const githubHostedLabels = [];
    const selfHostedLabels = [];
    const combinedLabels = [];
    
    Object.entries(labelStats).forEach(([labels, count]) => {
      const percentage = ((count / totalJobs) * 100).toFixed(1);
      const entry = `  ${labels}: ${count} jobs (${percentage}%)`;
      
      if (labels.includes('self-hosted')) {
        if (labels.includes(',')) {
          combinedLabels.push(entry);
        } else {
          selfHostedLabels.push(entry);
        }
      } else {
        githubHostedLabels.push(entry);
      }
    });
  }
  
  return {
    'stdout': JSON.stringify(data, null, 2),
  };
}

// Webhook URL
const webhookUrl = 'http://localhost:8080/webhook'; // Adjust as needed

// Shared Map to track running jobs across VUs
const runningJobs = new Map();

// Label sets for different runner types
const labelSets = {
  // 5 GitHub-hosted label sets (single labels, GitHub-hosted runners)
  githubHosted: [
    ['ubuntu-latest'],
    ['windows-latest'], 
    ['macos-latest'],
    ['ubuntu-20.04'],
    ['windows-2019']
  ],
  
  // 5 Self-hosted label sets (single labels, self-hosted runners)
  selfHosted: [
    ['self-hosted'],
    ['linux'],
    ['gpu'],
    ['large'],
    ['production']
  ],
  
  // 5 Combined label sets (multiple labels forming arrays)
  combined: [
    ['self-hosted', 'linux', 'x64'],
    ['self-hosted', 'gpu', 'cuda'],
    ['self-hosted', 'macos', 'arm64'],
    ['self-hosted', 'windows', 'dotnet'],
    ['self-hosted', 'production', 'high-memory']
  ]
};

const outcomes = ['success', 'success', 'success', 'success', 'failure', 'cancelled'];

// Helper function to get random label set
function getRandomLabels() {
  const types = ['githubHosted', 'selfHosted', 'combined'];
  const randomType = types[Math.floor(Math.random() * types.length)];
  const labelOptions = labelSets[randomType];
  return labelOptions[Math.floor(Math.random() * labelOptions.length)];
}

// Create a new workflow job webhook event payload
function createWebhookEvent(jobId, status, baseTime, runId) {
  const labels = getRandomLabels();
  
  // Track label usage for statistics
  if (status === 'queued') {
    trackLabelUsage(labels);
  }
  
  const now = new Date();
  const payload = {
    action: status,
    workflow_job: {
      id: jobId,
      name: `Job ${jobId}`,
      run_id: runId,
      labels: labels,
      created_at: baseTime.toISOString(),
    }
  };
  
  if (status === 'in_progress') {
    payload.workflow_job.started_at = now.toISOString();
  } else if (status === 'completed') {
    payload.workflow_job.started_at = runningJobs.get(jobId).startedAt;
    payload.workflow_job.completed_at = now.toISOString();
    payload.workflow_job.conclusion = outcomes[Math.floor(Math.random() * outcomes.length)];
  }
  
  
  return payload;
}

// Create a new workflow run webhook event payload
function createWorkflowRunEvent(runId, status, baseTime) {
  const repoNames = [
    'example-repo', 'test-project', 'demo-app', 'sample-service', 
    'api-backend', 'frontend-ui', 'mobile-app', 'data-pipeline',
    'ml-training', 'monitoring-stack'
  ];
  const workflowTitles = [
    'CI/CD Pipeline', 'Test Suite', 'Build and Deploy', 'Quality Check',
    'Security Scan', 'Performance Tests', 'Code Analysis', 'Release Build',
    'Integration Tests', 'Documentation Build'
  ];
  const randomRepo = repoNames[Math.floor(Math.random() * repoNames.length)];
  const randomTitle = workflowTitles[Math.floor(Math.random() * workflowTitles.length)];
  
  const now = new Date();
  const payload = {
    action: status,
    repository: {
      name: randomRepo,
      url: `https://github.com/example/${randomRepo}`
    },
    workflow_run: {
      id: runId,
      name: `Run ${runId}`,
      status: status,
      url: `https://github.com/example/${randomRepo}/actions/runs/${runId}`,
      display_title: randomTitle,
      created_at: baseTime.toISOString(),
    }
  };
  
  // Set conclusion and started_at based on status
  if (status === 'in_progress') {
    payload.workflow_run.run_started_at = now.toISOString();
  } else if (status === 'completed') {
    payload.workflow_run.run_started_at = runningJobs.get(runId).startedAt;
    payload.workflow_run.conclusion = outcomes[Math.floor(Math.random() * outcomes.length)];
  }
  
  return payload;
}

function generateDeliveryId() {
  return `delivery-${Math.floor(Math.random() * 1000000)}-${Date.now()}`;
}

function sendEvent(payload, event) {
  const payloadStr = `payload=${encodeURIComponent(JSON.stringify(payload))}`;
  const signature = generateSignature(payloadStr);

  const res = http.post(webhookUrl, payloadStr, {
    headers: { 
      'Content-Type': 'application/x-www-form-urlencoded',
      'X-Hub-Signature-256': signature,
      'X-GitHub-Event': event,
      'X-GitHub-Delivery': generateDeliveryId()
    },
  });

  check(res, { 
    "event status was 202": (r) => r.status == 202
  });
}

// Send a webhook event for workflow jobs
function sendWebhookEvent(jobId, status, baseTime, runId) {
  const payload = createWebhookEvent(jobId, status, baseTime, runId);
  sendEvent(payload, 'workflow_job');
}

// Send a webhook event for workflow runs
function sendWorkflowRunEvent(runId, status, baseTime) {
  const payload = createWorkflowRunEvent(runId, status, baseTime);
  sendEvent(payload, 'workflow_run');
}

function getRandomQueueTime() {
  const scenarios = [
    { weight: 0.3, min: 0.1, max: 1 },     // 30% fast jobs (0.1-1s)
    { weight: 0.4, min: 1, max: 10 },      // 40% normal jobs (1-10s)
    { weight: 0.2, min: 10, max: 60 },     // 20% slow jobs (10-60s)
    { weight: 0.1, min: 60, max: 300 }     // 10% very slow jobs (1-5min)
  ];
  
  const random = Math.random();
  let cumulative = 0;
  
  for (const scenario of scenarios) {
    cumulative += scenario.weight;
    if (random <= cumulative) {
      return Math.random() * (scenario.max - scenario.min) + scenario.min;
    }
  }
  
  return 5; // fallback
}

// Default function (main test scenario)
export default function() {
  const baseId = (parseInt(__VU) * 10000) + parseInt(__ITER);
  const jobId = baseId;
  const runId = baseId + 5000; // Offset run IDs to avoid conflicts
  const baseTime = new Date();

  // Decide if this iteration should also send workflow_run events (60% chance)
  const shouldSendWorkflowRun = Math.random() < 0.6;

  // First, queue the job
  sendWebhookEvent(jobId, 'queued', baseTime, runId);
  runningJobs.set(jobId, { status: 'queued', createdAt: baseTime });
  
  // If this iteration includes workflow run, also queue it
  if (shouldSendWorkflowRun) {
    sendWorkflowRunEvent(runId, 'queued', baseTime);
    runningJobs.set(runId, { status: 'queued', createdAt: baseTime });
  }
  
  const queueTime = getRandomQueueTime();
  sleep(queueTime);

  // Then move job to in_progress
  const startedAt = new Date();
  sendWebhookEvent(jobId, 'in_progress', baseTime, runId);
  runningJobs.set(jobId, { status: 'in_progress', startedAt: startedAt });
  
  // If this iteration includes workflow run, also move it to in_progress
  if (shouldSendWorkflowRun) {
    sendWorkflowRunEvent(runId, 'in_progress', baseTime);
    runningJobs.set(runId, { status: 'in_progress', startedAt: startedAt });
  }
  
  sleep(Math.random() * 3 + 2); // Random sleep 2-5 seconds

  // Finally complete the job
  sendWebhookEvent(jobId, 'completed', baseTime, runId);
  runningJobs.delete(jobId);
  
  // If this iteration includes workflow run, also complete it
  if (shouldSendWorkflowRun) {
    sendWorkflowRunEvent(runId, 'completed', baseTime);
    runningJobs.delete(runId);
  }
  
  // Add some variability between job iterations
  sleep(Math.random() * 2);
}