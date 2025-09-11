# Grafana Setup

This directory contains the Grafana configuration for monitoring GitHub Actions runners.

## Structure

```
grafana/
├── provisioning/
│   ├── datasources/
│   │   └── prometheus.yml      # Prometheus datasource configuration
│   └── dashboards/
│       ├── dashboard.yml       # Dashboard provider configuration
│       └── live-actions.json # Sample dashboard for GitHub Actions runners
```

## Configuration

### Datasource
- **Prometheus**: Automatically configured to connect to the Prometheus container at `http://prometheus:9090`

### Dashboard
- **GitHub Actions Runners Demand**: A sample dashboard showing:
  - Running jobs count (using `github_runners_jobs{job_status="running"}`)
  - Queued jobs count (using `github_runners_jobs{job_status="queued"}`)
  - Jobs over time graph
  - Average queue time (using `github_runners_average_queue_time_seconds`)
  - Peak demand total (using `github_runners_peak_demand_total`)
  - Queue duration histogram (using `github_runners_queue_duration_seconds_bucket`)

## Usage

1. Start the services:
   ```bash
   docker-compose up -d
   ```

2. Access Grafana at: http://localhost:3000

3. Default credentials:
   - Username: `admin`
   - Password: `admin` (or value from `GRAFANA_PASSWORD` environment variable)

4. The Prometheus datasource and sample dashboard will be automatically loaded.

## Customization

- Add more dashboards by placing JSON files in `grafana/provisioning/dashboards/`
- Modify the Prometheus datasource configuration in `grafana/provisioning/datasources/prometheus.yml`
- Update the dashboard configuration in `grafana/provisioning/dashboards/dashboard.yml`

## Environment Variables

- `GRAFANA_PASSWORD`: Set the admin password (default: `admin`)
