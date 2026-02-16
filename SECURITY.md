## Environment Variables for Production

### Required Environment Variables

```bash
# Security Configuration
WEBHOOK_SECRET=your-github-webhook-secret
ENVIRONMENT=production
TLS_ENABLED=true

# Server Configuration
PORT=8080
LOG_LEVEL=info

# Data Management Configuration
DATA_RETENTION_DAYS=30
CLEANUP_INTERVAL_HOURS=24
```

## Production Deployment Requirements

### 1. **HTTPS/TLS Configuration**
```bash
# Set environment variables
export ENVIRONMENT=production
export TLS_ENABLED=true
```

**Important**: The application does not implement TLS directly. The `TLS_ENABLED` flag controls secure cookie settings and security headers. For production deployments, use a reverse proxy (nginx, Traefik, CloudFlare) or load balancer for TLS termination.

### 2. **Database Security**
- SQLite database file is stored locally with restricted directory permissions (`0700`)
- Database path is configurable via `DATABASE_PATH` (default: `./data/live-actions.db`)

### 3. **Network Security**
- Deploy behind a WAF (Web Application Firewall)
- Use private networks for database connections
- Implement network-level rate limiting

### 4. **Application Security Features**
- **CSRF Protection**: All API endpoints include CSRF token validation
- **Origin Validation**: Dashboard API endpoints validate referer headers
- **Security Headers**: Comprehensive security headers applied automatically
- **Input Validation**: All user inputs are validated and sanitized
- **Request Logging**: Security-relevant events are logged with structured logging
- **Webhook Signature Validation**: GitHub webhook signatures are verified using HMAC-SHA256
