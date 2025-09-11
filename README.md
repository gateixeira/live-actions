[![GitHub Actions](https://img.shields.io/badge/GitHub-Actions-blue?logo=github-actions)](https://github.com/features/actions)
[![Go](https://img.shields.io/badge/Go-1.23+-blue?logo=go)](https://golang.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-Database-blue?logo=postgresql)](https://www.postgresql.org/)
[![Prometheus](https://img.shields.io/badge/Prometheus-Metrics-orange?logo=prometheus)](https://prometheus.io/)
[![Grafana](https://img.shields.io/badge/Grafana-Dashboard-orange?logo=grafana)](https://grafana.com/)
[![Docker](https://img.shields.io/badge/Docker-Container-blue?logo=docker)](https://www.docker.com/)

[![Beta](https://img.shields.io/badge/Status-Beta-yellow?style=for-the-badge)](https://github.com/gateixeira/live-actions/issues)

# Live Actions - GitHub Actions Monitoring 🚀

> ⚠️ **Beta Software Notice**: Live Actions is currently in beta. While functional and actively developed, expect potential instabilities. Please report issues and provide feedback to help us improve!

An application for live GitHub Actions Job and Runner monitoring integrated with Prometheus.

## Overview

**Live Actions** provides real-time monitoring and analytics for GitHub Actions workflow. Built with Go, PostgreSQL and Prometheus, it delivers comprehensive insights into your CI/CD infrastructure and works with GitHub Enterprise Cloud and Server at Enterprise, Organization and Repository levels.

### 🎯 **Core Features**

#### **📊 Interactive Dashboard**
- Live visualization of runner demand
- Historical data analysis
- Configurable tracking for GitHub-hosted vs self-hosted runners
- Visual status for queued, running, completed, and failed jobs

<img width="1378" height="724" alt="live-actions-dashboard" src="https://github.com/user-attachments/assets/40ca0198-994d-4f70-a181-4c30010d9885" />

#### **📋 Workflow Runs Management**
- Complete history of recent workflow executions with pagination
- Click to view individual job information for each workflow run
- Real-time status updates (queued, in_progress, completed, failed)

<img width="1319" height="827" alt="workflow-runs" src="https://github.com/user-attachments/assets/4739bfa3-075a-4ef2-9e62-d4335c9eb370" />

#### **🏷️ Job Label Analytics**
- Configurable detection of GitHub-hosted vs self-hosted runners
- Running, queued, completed, and total counts per label combination
- Live refresh of label-based metrics

<img width="1315" height="429" alt="jobs-by-label" src="https://github.com/user-attachments/assets/716abeb1-675b-4f89-9b0f-f9cb244fc27e" />

#### **⚡ Runner Analytics**
- Monitor workflow queue times and peak demand periods

## 🔥 **Live Actions vs GitHub's Built-in Metrics**

While GitHub offers [Actions Usage Metrics](https://docs.github.com/en/enterprise-cloud@latest/organizations/collaborating-with-groups-in-organizations/viewing-github-actions-metrics-for-your-organization#about-github-actions-usage-metrics) and [Actions Performance Metrics](https://docs.github.com/en/enterprise-cloud@latest/organizations/collaborating-with-groups-in-organizations/viewing-github-actions-metrics-for-your-organization#about-github-actions-performance-metrics), Live Actions provides **real-time operational monitoring**.

### **📈 Real-time vs Historical Reporting**

| Feature | GitHub's Metrics | Live Actions |
|---------|------------------|------------------|
| **Job Status Tracking** | Completed job analysis | **Live job status** (queued → in_progress → completed) |
| **Update Frequency** | Periodic reporting | **Instant updates** as jobs change state |
| **Queue Monitoring** | Retrospective queue times | **Real-time queue tracking** and demand spikes |

### **⚡ Live Operational Intelligence**

**Live Actions excels at answering operational questions:**

- 🔴 **"How many jobs are queued RIGHT NOW?"** - Live queue depth monitoring
- 🟡 **"Which runners are currently overwhelmed?"** - Real-time capacity analysis  
- 🟢 **"Is there a job surge happening?"** - Live demand spike detection
- ⏱️ **"How long are jobs waiting in queue today?"** - Current queue time trends

**The bottom line:** Live Actions and GitHub's metrics serve different but complementary purposes. Use Live Actions for **real-time operations** and GitHub's metrics for **strategic planning and optimization**.

## Requirements

- **Go 1.23+** - Latest Go version with modern features and security enhancements
- **PostgreSQL** - For data storage
- **Prometheus** - For metrics collection and monitoring
- **Environment Variables** - See configuration section below

## Installation

### Using Published Docker Container

The easiest way to get started is through Docker Compose:

Before running the container, make sure you set all environment variables in an `.env` file or directly in your shell (use `.env.example` as a template):

- `DATABASE_URL`: PostgreSQL connection string (e.g., `postgresql://user:password@host:5432/dbname?sslmode=disable`)
- `WEBHOOK_SECRET`: Secret for GitHub webhook validation
- `PORT`: defaults to 8080
- `LOG_LEVEL`: defaults: info
- `GIN_MODE`: Gin framework mode (default: release)
- `ENVIRONMENT`: application environment (default: development)
- `TLS_ENABLED`: enable HTTPS features (default: false)
- `DATA_RETENTION_DAYS`: data retention period (default: 30 days)
- `CLEANUP_INTERVAL_HOURS`: cleanup checks for expired retention (default: 24h)
- `PROMETHEUS_URL`: Prometheus server URL (default: http://prometheus:9090 to match docker-compose setup)
- `RUNNER_TYPE_CONFIG_PATH`: runner config path (default: config/runner_types.json)

#### **Running with Docker Compose**

```bash
make docker-run-remote # Start the application with docker-compose
```

This will start:

- The main application container
- A PostgreSQL instance
- A Prometheus monitoring server (available at `http://localhost:9090`)
- **[For demo purposes]** A Grafana dashboard server (available at `http://localhost:3000`)

Open your browser and navigate to `http://localhost:8080/dashboard` to access the interactive dashboard.

A `/metrics` endpoint is also available for Prometheus integration.

To start receiving GitHub webhooks, check the configuration section below under **GitHub Webhook Configuration**.

### Local Development

To build the application locally, run:

```bash
make docker-build
```

Then run the application with:

```bash
make docker-run
```

There are several other make commands available:

```bash
make build    # Build the live-actions binary
make run      # Run the application
make test     # Run tests
make clean    # Clean build files
make lint     # Run linter
make deps     # Install dependencies
```

The server will start on port 8080.

## API Endpoints

- `GET /` - Health check and application root
- `GET /dashboard` - Interactive dashboard UI with real-time monitoring
- `GET /metrics` - Prometheus metrics endpoint for observability integration
- `GET /events` - Server-Sent Events (SSE) endpoint for real-time updates

## Webhook Security

The webhook endpoint implements enterprise-grade security using GitHub's webhook signature validation:

### **GitHub Webhook Configuration**

When configuring your GitHub webhook:

1. **Generate a secure webhook secret**:
   ```bash
   # Generate a cryptographically secure secret
   openssl rand -hex 32
   ```

2. **Set the secret in your environment**:
   ```bash
   export WEBHOOK_SECRET=your_generated_secret_here
   ```

3. **Configure the GitHub webhook**:
   - Payload URL: `https://your-domain.com/webhook`
   - Secret: Use the secret from step 1
   - Events: Select "Workflow jobs" and "Workflow runs" under "Individual events"
   - Active: ✅ Enabled

### **Security Validation**
GitHub includes a signature header (`X-Hub-Signature-256`) with each webhook request. The application validates this signature before processing any webhook data, ensuring requests originate from GitHub and haven't been tampered with.

### **Local Development with ngrok**

For local development and testing:

1. **Install ngrok**: Download from [ngrok.com](https://ngrok.com/download)

2. **Start your application**:
   ```bash
   make run
   # or
   docker compose up
   ```

3. **Expose your local server**:
   ```bash
   ngrok http 8080
   ```

4. **Configure your webhook**:
   - Copy the ngrok HTTPS URL (e.g., `https://a1b2c3d4.ngrok.io`)
   - Update your GitHub webhook payload URL to: `https://a1b2c3d4.ngrok.io/webhook`
   - Set your webhook secret in your environment:
     ```bash
     export WEBHOOK_SECRET=your_generated_secret_here
     ```

5. **Restart your application** to apply the new secret

**Note**: Free ngrok URLs change on restart. Update your webhook URL in GitHub when this happens, or consider ngrok's paid plans for persistent URLs.

### **Runner Type Detection & Configuration**

Live Actions classifies runners as either GitHub-hosted or self-hosted based on job labels using a configurable detection system.

#### **Configuration File**

Runner type detection is controlled by the `config/runner_types.json` file.

#### **Detection Logic**

The runner type inference follows this priority order:

1. **Self-hosted Priority**: If any job label matches `self_hosted_labels`, the job is classified as self-hosted
2. **GitHub-hosted Detection**: If any job label matches `github_hosted_labels`, the job is classified as GitHub-hosted  
3. **Default Fallback**: If no labels match, uses the `default_runner_type` (typically "unknown")

#### **Configuration Options**

- `RUNNER_TYPE_CONFIG_PATH`: Path to the runner types configuration file (default: `config/runner_types.json`)

#### **Example Scenarios**

| Job `runs-on` Labels | Classification | Reason |
|---------------------|----------------|---------|
| `["ubuntu-latest"]` | GitHub-hosted | Matches `github_hosted_labels` |
| `["self-hosted", "linux"]` | Self-hosted | Contains explicit `self-hosted` label |
| `["custom", "gpu"]` | Self-hosted | No matches, uses default |
| `["ubuntu-latest", "self-hosted"]` | Self-hosted | Self-hosted takes priority |

This approach handles complex scenarios where jobs might have custom labels but still run on GitHub-hosted infrastructure, or where self-hosted runners pick up jobs that don't explicitly declare `self-hosted` in their `runs-on` configuration.

## Testing

You can manually test the application by visiting:

- **Dashboard**: `http://localhost:8080/dashboard`
- **Prometheus Metrics**: `http://localhost:8080/metrics`
- **Grafana Dashboard**: `http://localhost:3000` (access: admin/admin)

## Data Retention & Cleanup

Live Actions includes automatic data cleanup functionality to manage database size and ensure optimal performance over time.

### **Automatic Data Retention**

The application automatically removes old workflow data based on configurable retention policies:

- **Default Retention**: 30 days (configurable via `DATA_RETENTION_DAYS`)
- **Cleanup Frequency**: Daily at startup and every 24 hours (configurable via `CLEANUP_INTERVAL_HOURS`)
- **Data Types Cleaned**: Both workflow runs and workflow jobs older than the retention period
- **Background Operation**: Cleanup runs in the background without affecting application performance

## Monitoring & Observability

### **Prometheus Metrics Endpoint**

Live Actions exposes comprehensive metrics through a Prometheus-compatible endpoint at `/metrics`, providing deep insights into your GitHub Actions infrastructure performance.

### **Built-in Grafana Integration**

<img width="1466" height="1088" alt="live-actions-grafana" src="https://github.com/user-attachments/assets/ef8a4aab-b5d1-4cef-b8df-5650b3d72d35" />

For demo purposes, when using Docker Compose, Live Actions includes a pre-configured Grafana instance configured to connect to the Prometheus monitoring stack

- **Default login**: admin/admin (configurable via `GRAFANA_PASSWORD`)
- **Access URL**: `http://localhost:3000`
- **Pre-built dashboard**: configured from `grafana/provisioning/dashboards/live-actions.json`

### **Built-in Prometheus Integration**

Live Actions requires a Prometheus server for monitoring and includes a pre-configured Prometheus server when started with Docker Compose. The server:

- **Scrapes metrics** every 10 seconds from the application
- **Stores historical data** with persistent volumes
- **Provides query interface** at `http://localhost:9090`
- **Enables alerting** capabilities for production monitoring

#### **Prometheus Configuration**

The included `prometheus.yml` configuration:

```yaml
global:
  scrape_interval: 5s
  evaluation_interval: 5s

scrape_configs:
  - job_name: 'actions-runner-monitor'
    static_configs:
      - targets: ['app-runner:8080']
    scrape_interval: 10s
    metrics_path: /metrics
```

This setup makes Live Actions compatible with popular observability tools:

#### **Common Integrations**
- **Datadog**: Use Prometheus integration to ingest metrics
- **New Relic**: Configure Prometheus remote write
- **Splunk**: Forward metrics via Prometheus federation
- **Cloud Monitoring**: Export to AWS CloudWatch, Azure Monitor, or GCP Monitoring

## Limitations

While Live Actions provides powerful real-time monitoring capabilities, there are some limitations to be aware of:

### **Data Accuracy & Timing**
- **Metrics Reconciliation Delays**: Although the application tracks metrics in real-time, there may be slight delays in the reconciliation process due to webhook processing and database operations.
- **GitHub Webhook Reliability**: We've observed scenarios where GitHub may fail to send events for completed workflow runs, which can result in incomplete data for certain workflows.
- **Event Ordering**: GitHub does not guarantee the order of webhook events, so event reordering is handled on a best-effort basis using timestamps and status transitions.

### **Runner Type Detection**
- **Best-Effort Classification**: It is not possible to confidently recognize whether a job is running on a self-hosted or GitHub-hosted runner based on job labels alone. Since the application does not integrate with the GitHub API, runner type tagging is performed on a best-effort basis using the `runner_types.json` configuration file.
- **Label-Based Limitations**: Custom runner labels or non-standard configurations may not be correctly classified without manual configuration updates.
