## Environment Variables for Production

### Required Environment Variables

```bash
# Database Configuration
DATABASE_URL=postgresql://live_actions_user:your-secure-password@your-production-db-host:5432/live_actions?sslmode=require

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

# Monitoring Configuration
PROMETHEUS_URL=http://localhost:9090

# Runner Configuration
RUNNER_TYPE_CONFIG_PATH=config/runner_types.json
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
- Use SSL/TLS for database connections (automatically enabled in production)
- Create dedicated database user with minimal privileges
- Enable database audit logging

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

### 5. **Monitoring & Alerting**
- Alert on suspicious request patterns
- Track failed authentication attempts

### 6. **Regular Security Updates**
```bash
# Keep dependencies updated
go mod tidy
go get -u all
```

## Security Incident Response

### Suspected Attack Response
1. Check logs: `docker logs <container_id> | grep "security"`
2. Monitor for unusual request patterns in structured logs
3. Check for unusual database activity
4. Rotate webhook secrets if compromised
5. Review security headers and CSRF token validation logs

### Log Analysis
```bash
# Check application logs (structured JSON format)
docker logs <container_id> | grep "level\":\"error"
docker logs <container_id> | grep "security"

# Check for webhook validation failures
docker logs <container_id> | grep "Webhook validation failed"

# Check for suspicious requests
docker logs <container_id> | grep "Access denied"
```
