# AlertManager Integration Summary

This document provides an overview of the AlertManager integration in CryptoFunk.

## Overview

AlertManager is fully integrated into CryptoFunk's monitoring stack, providing intelligent alert routing and multi-channel notifications for critical trading system events.

## Architecture

```
Prometheus
    ↓ (evaluates alert rules)
Alert Rules (deployments/prometheus/alerts/*.yml)
    ↓ (fires alerts)
AlertManager (deployments/prometheus/alertmanager.yml)
    ↓ (routes and groups)
Notification Channels:
    ├─ Slack (#cryptofunk-alerts, #cryptofunk-critical, etc.)
    ├─ Email (SMTP)
    └─ AlertManager UI (http://localhost:9093)
```

## Components

### 1. AlertManager Configuration

**Location**: `/deployments/prometheus/alertmanager.yml`

**Features**:
- **Route-based alert dispatching** by severity and type
- **Intelligent grouping** to reduce notification noise
- **Inhibition rules** to prevent duplicate alerts
- **Multiple receivers** for different alert categories
- **Configurable timing** (group_wait, group_interval, repeat_interval)

**Receivers**:
- `team-notifications` - Default receiver (Slack + Email)
- `critical-alerts` - Critical alerts with @channel mentions
- `circuit-breaker-alerts` - Circuit breaker events
- `trading-alerts` - Trading-specific alerts
- `agent-alerts` - Agent health alerts
- `system-alerts` - System resource alerts

### 2. Alert Rules

**Location**: `/deployments/prometheus/alerts/`

**Categories**:
- **Agent Alerts** (`agent_alerts.yml`) - Agent health, latency, confidence
- **Trading Alerts** (`trading_alerts.yml`) - Drawdown, win rate, P&L, execution
- **System Alerts** (`system_alerts.yml`) - CPU, memory, disk usage
- **Error Alerts** (`error_alerts.yml`) - HTTP errors, API failures
- **Health Alerts** (`health.yml`) - Service health checks
- **Vector Search Alerts** (`vector_search_alerts.yml`) - pgvector performance

### 3. Docker Compose Integration

**File**: `/docker-compose.yml`

AlertManager service includes:
- Port 9093 exposed for UI access
- Health checks for reliability
- Volume for persistent alert state
- Environment variables for notification config

### 4. Kubernetes Integration

**Files**:
- `/deployments/k8s/base/deployment-alertmanager.yaml` - Deployment manifest
- `/deployments/k8s/base/configmap-monitoring.yaml` - Configuration
- `/deployments/k8s/base/pvc.yaml` - Persistent storage
- `/deployments/k8s/base/services.yaml` - Service exposure

**Features**:
- LoadBalancer service for external access
- Secret-based configuration
- Persistent volume for alert state
- Health probes (liveness + readiness)
- Resource limits and requests

## Configuration

### Environment Variables

Add to `.env` file:

```bash
# Slack webhook
SLACK_WEBHOOK_URL=https://hooks.slack.com/services/YOUR/WEBHOOK/URL

# SMTP configuration
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_FROM=cryptofunk-alerts@example.com
SMTP_USERNAME=your-email@example.com
SMTP_PASSWORD=your-app-password

# Alert recipients
ALERT_EMAIL_TO=team@example.com
CRITICAL_ALERT_EMAIL_TO=oncall@example.com
```

### Kubernetes Secrets

Update `/deployments/k8s/base/secrets.yaml`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: cryptofunk-secrets
  namespace: cryptofunk
stringData:
  slack-webhook-url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
  smtp-host: "smtp.gmail.com"
  smtp-port: "587"
  smtp-from: "cryptofunk-alerts@example.com"
  smtp-username: "your-email@example.com"
  smtp-password: "your-app-password"
  alert-email-to: "team@example.com"
  critical-alert-email-to: "oncall@example.com"
```

## Usage

### Starting AlertManager

**Docker Compose**:
```bash
docker-compose up -d alertmanager
```

**Kubernetes**:
```bash
kubectl apply -f deployments/k8s/base/deployment-alertmanager.yaml
```

### Accessing AlertManager UI

**Docker Compose**:
```bash
# Open browser
open http://localhost:9093

# Or use Task
task alertmanager
```

**Kubernetes**:
```bash
# Port forward
kubectl port-forward -n cryptofunk svc/alertmanager-service 9093:9093

# Or access via LoadBalancer
kubectl get svc -n cryptofunk alertmanager-service
```

### Testing Alerts

**Quick Test**:
```bash
# Using Task
task test-alerts
```

**Manual Test**:
```bash
# Send test alert
curl -X POST http://localhost:9093/api/v2/alerts \
  -H "Content-Type: application/json" \
  -d '[{
    "labels": {
      "alertname": "TestAlert",
      "severity": "info"
    },
    "annotations": {
      "summary": "Test alert",
      "description": "Testing AlertManager"
    }
  }]'

# View alerts
curl http://localhost:9093/api/v2/alerts | jq
```

## Alert Routing

### Severity-Based Routing

| Severity | Receiver | Slack Channel | Email | Response Time |
|----------|----------|---------------|-------|---------------|
| `critical` | critical-alerts | #cryptofunk-critical (@channel) | oncall@example.com | 15 min |
| `warning` | team-notifications | #cryptofunk-alerts | team@example.com | 1 hour |
| `info` | team-notifications | #cryptofunk-alerts | team@example.com | Best effort |

### Alert-Specific Routing

| Alert Pattern | Receiver | Slack Channel |
|---------------|----------|---------------|
| `CircuitBreakerOpen` | circuit-breaker-alerts | #cryptofunk-ops |
| `HighErrorRate`, `TradingSessionFailure` | trading-alerts | #cryptofunk-trading |
| `AgentDown`, `AgentHighLatency` | agent-alerts | #cryptofunk-agents |
| `HighMemoryUsage`, `HighCPUUsage` | system-alerts | #cryptofunk-ops |

### Grouping and Timing

- **Group Wait**: 10s (wait before first notification)
- **Group Interval**: 5m (wait before sending new batch)
- **Repeat Interval**: 4h (repeat if not resolved)
- **Critical Alerts**: No group wait, repeat every 1h

## Alert Lifecycle

1. **Prometheus evaluates rules** (every 15s)
2. **Alert enters pending state** (waits for `for` duration)
3. **Alert fires** (condition met for duration)
4. **AlertManager receives alert**
5. **Alert is grouped** with similar alerts
6. **Alert is routed** to appropriate receiver(s)
7. **Notifications sent** via Slack/Email
8. **Alert resolved** (condition no longer met)
9. **Resolution notification sent**

## Inhibition Rules

Prevent notification spam:

1. **Critical inhibits Warning** - Don't send warning if critical is firing
   ```yaml
   source_match:
     severity: 'critical'
   target_match:
     severity: 'warning'
   ```

2. **Orchestrator Down inhibits Agent alerts** - Don't alert on agents if orchestrator is down
   ```yaml
   source_match:
     alertname: 'OrchestratorDown'
   target_match_re:
     alertname: 'Agent.*'
   ```

## Notification Templates

### Slack Message Format

```
[SEVERITY] Alert Name
Summary: <alert summary>
Description: <alert description>
Environment: production
Started: <timestamp>
```

### Email Format

```
Subject: [CryptoFunk] AlertName - FIRING

AlertName
Status: firing
Severity: critical
Environment: production

Summary: <summary>
Description: <description>
Started: <timestamp>
```

## Monitoring AlertManager

### Health Checks

```bash
# Health status
curl http://localhost:9093/-/healthy

# Readiness check
curl http://localhost:9093/-/ready

# Configuration
curl http://localhost:9093/api/v2/status | jq
```

### Metrics

AlertManager exposes Prometheus metrics:

```
alertmanager_notifications_total
alertmanager_notifications_failed_total
alertmanager_notification_latency_seconds
alertmanager_alerts_received_total
alertmanager_alerts_invalid_total
```

### Logs

```bash
# Docker Compose
docker-compose logs -f alertmanager

# Kubernetes
kubectl logs -f -n cryptofunk deployment/alertmanager
```

## Troubleshooting

### Common Issues

1. **Alerts not appearing**
   - Check Prometheus -> AlertManager connectivity
   - Verify AlertManager is receiving alerts: `curl http://localhost:9093/api/v2/alerts`

2. **Slack notifications not delivered**
   - Verify webhook URL is correct
   - Test webhook directly
   - Check AlertManager logs for errors

3. **Email not delivered**
   - Verify SMTP credentials
   - Check spam folder
   - Test SMTP connection with swaks

4. **Alert rules not loading**
   - Validate YAML syntax
   - Check Prometheus configuration
   - Verify rule files are mounted

See `/docs/ALERTMANAGER_TESTING.md` for detailed troubleshooting.

## Security Considerations

1. **Webhook URLs** - Never commit to git, use environment variables
2. **SMTP passwords** - Use app passwords, not account passwords
3. **AlertManager UI** - Secure with authentication in production
4. **Network policies** - Restrict AlertManager access
5. **TLS/SSL** - Use HTTPS for webhooks and SMTP

## Performance

### Resource Usage

**Docker Compose**:
- CPU: 100m - 500m
- Memory: 256Mi - 512Mi
- Storage: 5Gi

**Kubernetes**:
```yaml
resources:
  requests:
    cpu: "100m"
    memory: "256Mi"
  limits:
    cpu: "500m"
    memory: "512Mi"
```

### Scaling Considerations

- AlertManager is stateful (stores silences, alert state)
- For HA, use multiple replicas with gossip clustering
- Use persistent storage for alert state
- Consider using Prometheus Alertmanager HA setup

## Best Practices

1. **Start with Paper Trading** - Test alerts before live trading
2. **Test regularly** - Run `task test-alerts` weekly
3. **Document alert responses** - Use `/docs/ALERT_RUNBOOK.md`
4. **Monitor notification success** - Track `alertmanager_notifications_failed_total`
5. **Use silences wisely** - Silence during maintenance windows
6. **Review and tune** - Adjust thresholds based on false positive rate
7. **Set up escalation** - Define clear escalation paths
8. **Keep runbook updated** - Document all alert responses

## Related Documentation

- **Alert Runbook**: `/docs/ALERT_RUNBOOK.md` - Response procedures for each alert
- **Testing Guide**: `/docs/ALERTMANAGER_TESTING.md` - Comprehensive testing instructions
- **Metrics Guide**: `/docs/METRICS_INTEGRATION.md` - Prometheus metrics reference
- **Production Checklist**: `/docs/PRODUCTION_CHECKLIST.md` - Pre-deployment checklist

## Quick Reference

### URLs

- **AlertManager UI**: http://localhost:9093
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000

### Commands

```bash
# Test alerts
task test-alerts

# Open AlertManager
task alertmanager

# Send test alert
curl -X POST http://localhost:9093/api/v2/alerts \
  -H "Content-Type: application/json" \
  -d '[{"labels":{"alertname":"Test","severity":"info"},"annotations":{"summary":"Test"}}]'

# View alerts
curl http://localhost:9093/api/v2/alerts | jq

# Create silence
curl -X POST http://localhost:9093/api/v2/silences \
  -H "Content-Type: application/json" \
  -d '<silence-json>'

# Check health
curl http://localhost:9093/-/healthy
```

## Support

For issues or questions:
1. Check `/docs/ALERTMANAGER_TESTING.md` troubleshooting section
2. Review AlertManager logs
3. Consult Alert Runbook for specific alerts
4. Test notification channels independently

## Version History

- **v1.0** - Initial AlertManager integration
  - Docker Compose support
  - Kubernetes manifests
  - Multi-channel notifications (Slack, Email)
  - Comprehensive alert rules
  - Testing scripts and documentation
