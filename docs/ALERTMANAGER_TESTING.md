# AlertManager Integration Testing Guide

This guide provides comprehensive instructions for testing the AlertManager integration in CryptoFunk.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Manual Testing](#manual-testing)
- [Automated Testing](#automated-testing)
- [Testing Notification Channels](#testing-notification-channels)
- [Troubleshooting](#troubleshooting)

## Overview

CryptoFunk uses Prometheus AlertManager to route and deliver alerts to multiple notification channels:

- **Slack**: Real-time alerts to dedicated channels
- **Email**: SMTP-based email notifications
- **AlertManager UI**: Web interface for managing alerts and silences

The AlertManager configuration is in:
- **Docker Compose**: `/deployments/prometheus/alertmanager.yml`
- **Kubernetes**: `/deployments/k8s/base/configmap-monitoring.yaml`

## Prerequisites

### For Docker Compose Testing

1. **Services running**:
   ```bash
   docker-compose up -d prometheus alertmanager
   ```

2. **Environment variables configured** in `.env`:
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

### For Kubernetes Testing

1. **Secrets configured** in `/deployments/k8s/base/secrets.yaml`:
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: cryptofunk-secrets
     namespace: cryptofunk
   type: Opaque
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

2. **Deploy AlertManager**:
   ```bash
   kubectl apply -f deployments/k8s/base/
   ```

## Quick Start

### Using Task

The easiest way to test AlertManager:

```bash
# Run automated test suite
task test-alerts

# Open AlertManager UI
task alertmanager
```

### Manual Test

Send a test alert to AlertManager:

```bash
# For Docker Compose
curl -X POST http://localhost:9093/api/v2/alerts \
  -H "Content-Type: application/json" \
  -d '[{
    "labels": {
      "alertname": "TestAlert",
      "severity": "info",
      "component": "test"
    },
    "annotations": {
      "summary": "Test alert from manual test",
      "description": "Verifying AlertManager configuration"
    }
  }]'

# For Kubernetes
kubectl port-forward -n cryptofunk svc/alertmanager-service 9093:9093
# Then run the curl command above
```

## Manual Testing

### 1. Verify Services are Healthy

**Docker Compose**:
```bash
# Check Prometheus
curl http://localhost:9090/-/healthy

# Check AlertManager
curl http://localhost:9093/-/healthy

# View docker logs
docker-compose logs alertmanager
```

**Kubernetes**:
```bash
# Check pods
kubectl get pods -n cryptofunk -l app.kubernetes.io/name=alertmanager

# Check logs
kubectl logs -n cryptofunk deployment/alertmanager

# Check service
kubectl get svc -n cryptofunk alertmanager-service
```

### 2. Check AlertManager Configuration

```bash
# View loaded configuration
curl http://localhost:9093/api/v2/status | jq '.config'

# List configured receivers
curl http://localhost:9093/api/v2/status | jq '.config.receivers[]?.name'
```

Expected receivers:
- `team-notifications` (default)
- `critical-alerts`
- `circuit-breaker-alerts`
- `trading-alerts`
- `agent-alerts`
- `system-alerts`

### 3. Verify Prometheus Alert Rules

```bash
# List all loaded rules
curl http://localhost:9090/api/v1/rules | jq '.data.groups[]?.rules[]?.name'

# Check specific rule
curl http://localhost:9090/api/v1/rules | jq '.data.groups[]?.rules[]? | select(.name == "CircuitBreakerTripped")'
```

### 4. Check Current Alerts

```bash
# View firing alerts in Prometheus
curl http://localhost:9090/api/v1/alerts | jq '.data.alerts[]? | select(.state == "firing")'

# View alerts in AlertManager
curl http://localhost:9093/api/v2/alerts | jq '.[] | {name: .labels.alertname, status: .status.state}'
```

### 5. Send Test Alerts

**Info Alert** (goes to team-notifications):
```bash
curl -X POST http://localhost:9093/api/v2/alerts \
  -H "Content-Type: application/json" \
  -d '[{
    "labels": {
      "alertname": "TestInfoAlert",
      "severity": "info",
      "component": "test",
      "environment": "testing"
    },
    "annotations": {
      "summary": "Info level test alert",
      "description": "Testing info severity routing"
    },
    "startsAt": "'$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")'"
  }]'
```

**Warning Alert** (goes to team-notifications):
```bash
curl -X POST http://localhost:9093/api/v2/alerts \
  -H "Content-Type: application/json" \
  -d '[{
    "labels": {
      "alertname": "TestWarningAlert",
      "severity": "warning",
      "component": "test",
      "environment": "testing"
    },
    "annotations": {
      "summary": "Warning level test alert",
      "description": "Testing warning severity routing"
    },
    "startsAt": "'$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")'"
  }]'
```

**Critical Alert** (goes to critical-alerts AND team-notifications):
```bash
curl -X POST http://localhost:9093/api/v2/alerts \
  -H "Content-Type: application/json" \
  -d '[{
    "labels": {
      "alertname": "TestCriticalAlert",
      "severity": "critical",
      "component": "test",
      "environment": "testing"
    },
    "annotations": {
      "summary": "CRITICAL test alert",
      "description": "Testing critical severity routing with @channel mention"
    },
    "startsAt": "'$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")'"
  }]'
```

**Circuit Breaker Alert** (goes to circuit-breaker-alerts):
```bash
curl -X POST http://localhost:9093/api/v2/alerts \
  -H "Content-Type: application/json" \
  -d '[{
    "labels": {
      "alertname": "CircuitBreakerOpen",
      "severity": "warning",
      "component": "risk",
      "service": "exchange"
    },
    "annotations": {
      "summary": "Test circuit breaker alert",
      "description": "Exchange circuit breaker is open"
    },
    "startsAt": "'$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")'"
  }]'
```

### 6. Verify Alert Delivery

After sending test alerts:

1. **Check AlertManager UI**: http://localhost:9093
2. **Check Slack channels**:
   - `#cryptofunk-alerts` - Info and warning alerts
   - `#cryptofunk-critical` - Critical alerts with @channel
   - `#cryptofunk-ops` - Circuit breaker and system alerts
3. **Check email inboxes** for configured recipients

### 7. Test Alert Silencing

Silence a test alert:

```bash
curl -X POST http://localhost:9093/api/v2/silences \
  -H "Content-Type: application/json" \
  -d '{
    "matchers": [
      {
        "name": "alertname",
        "value": "TestInfoAlert",
        "isRegex": false,
        "isEqual": true
      }
    ],
    "startsAt": "'$(date -u +"%Y-%m-%dT%H:%M:%S.000Z")'",
    "endsAt": "'$(date -u -d '+1 hour' +"%Y-%m-%dT%H:%M:%S.000Z")'",
    "createdBy": "manual-test",
    "comment": "Silencing test alert"
  }'
```

View active silences:
```bash
curl http://localhost:9093/api/v2/silences | jq '.[] | {id: .id, comment: .comment, state: .status.state}'
```

Delete a silence:
```bash
# Get silence ID from above command
curl -X DELETE http://localhost:9093/api/v2/silence/{silenceID}
```

## Automated Testing

### Full Integration Test

Run the comprehensive test suite:

```bash
./scripts/test-alerts.sh
```

This script:
1. Checks Prometheus and AlertManager health
2. Verifies configuration
3. Lists loaded alert rules
4. Shows current firing alerts
5. Sends a test alert
6. Verifies alert appears in AlertManager
7. Checks alert routing
8. Silences test alert
9. Provides summary

### Expected Output

```
==============================================================================
CryptoFunk AlertManager Integration Test
==============================================================================

Step 1: Checking service health

Checking Prometheus... OK
Checking AlertManager... OK

Step 2: Verifying AlertManager configuration

Checking AlertManager config... OK

Configured receivers:
  - team-notifications
  - critical-alerts
  - circuit-breaker-alerts
  - trading-alerts
  - agent-alerts
  - system-alerts

Step 3: Checking Prometheus alert rules

Retrieving loaded alert rules... 24 rules loaded

...

==============================================================================
AlertManager Integration Test: PASSED
==============================================================================
```

## Testing Notification Channels

### Slack Webhook Setup

1. **Create Slack App**:
   - Go to https://api.slack.com/apps
   - Click "Create New App" > "From scratch"
   - Name: "CryptoFunk Alerts"
   - Choose workspace

2. **Enable Incoming Webhooks**:
   - In app settings, click "Incoming Webhooks"
   - Toggle "Activate Incoming Webhooks" to On
   - Click "Add New Webhook to Workspace"
   - Select channels:
     - `#cryptofunk-alerts`
     - `#cryptofunk-critical`
     - `#cryptofunk-trading`
     - `#cryptofunk-agents`
     - `#cryptofunk-ops`

3. **Copy Webhook URL**:
   - Format: `https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX`
   - Add to `.env` as `SLACK_WEBHOOK_URL`

4. **Test Slack**:
   ```bash
   curl -X POST "${SLACK_WEBHOOK_URL}" \
     -H "Content-Type: application/json" \
     -d '{
       "text": "Test message from CryptoFunk AlertManager setup",
       "channel": "#cryptofunk-alerts"
     }'
   ```

### Email SMTP Setup

#### Gmail Configuration

1. **Enable 2FA** on your Google account
2. **Create App Password**:
   - Go to https://myaccount.google.com/apppasswords
   - Select "Mail" and your device
   - Generate password (16 characters)

3. **Configure in `.env`**:
   ```bash
   SMTP_HOST=smtp.gmail.com
   SMTP_PORT=587
   SMTP_FROM=your-email@gmail.com
   SMTP_USERNAME=your-email@gmail.com
   SMTP_PASSWORD=your-16-char-app-password
   ```

#### SendGrid Configuration

1. **Create SendGrid Account**: https://signup.sendgrid.com/
2. **Create API Key**: Settings > API Keys
3. **Configure in `.env`**:
   ```bash
   SMTP_HOST=smtp.sendgrid.net
   SMTP_PORT=587
   SMTP_FROM=alerts@yourdomain.com
   SMTP_USERNAME=apikey
   SMTP_PASSWORD=SG.your-api-key
   ```

#### AWS SES Configuration

1. **Verify Domain** in AWS SES Console
2. **Create SMTP Credentials**: Account Dashboard > SMTP Settings
3. **Configure in `.env`**:
   ```bash
   SMTP_HOST=email-smtp.us-east-1.amazonaws.com
   SMTP_PORT=587
   SMTP_FROM=alerts@yourdomain.com
   SMTP_USERNAME=your-smtp-username
   SMTP_PASSWORD=your-smtp-password
   ```

### Test Email Delivery

```bash
# Install swaks (SMTP test tool)
# macOS: brew install swaks
# Ubuntu: apt-get install swaks

# Test SMTP connection
swaks --to team@example.com \
  --from cryptofunk-alerts@example.com \
  --server smtp.gmail.com:587 \
  --auth LOGIN \
  --auth-user your-email@gmail.com \
  --auth-password your-app-password \
  --tls \
  --header "Subject: Test Email from CryptoFunk" \
  --body "Testing SMTP configuration for AlertManager"
```

## Troubleshooting

### AlertManager Not Starting

**Symptom**: AlertManager container/pod fails to start

**Solutions**:
```bash
# Check logs
docker-compose logs alertmanager
# OR
kubectl logs -n cryptofunk deployment/alertmanager

# Common issues:
# 1. Invalid YAML in alertmanager.yml
# 2. Missing environment variables
# 3. Port 9093 already in use
```

### Alerts Not Appearing

**Symptom**: Test alerts sent but not visible in AlertManager

**Check**:
1. AlertManager is receiving alerts:
   ```bash
   curl http://localhost:9093/api/v2/alerts
   ```

2. Prometheus can reach AlertManager:
   ```bash
   curl http://localhost:9090/api/v1/alertmanagers
   ```

3. Network connectivity (Docker/K8s):
   ```bash
   # Docker
   docker-compose exec prometheus wget -O- http://alertmanager:9093/-/healthy

   # Kubernetes
   kubectl exec -n cryptofunk deployment/prometheus -- wget -O- http://alertmanager-service:9093/-/healthy
   ```

### Slack Notifications Not Delivered

**Symptom**: Alerts visible in AlertManager but not in Slack

**Check**:
1. Webhook URL is correct:
   ```bash
   echo $SLACK_WEBHOOK_URL
   # Should start with https://hooks.slack.com/services/
   ```

2. Test webhook directly:
   ```bash
   curl -X POST "${SLACK_WEBHOOK_URL}" \
     -H "Content-Type: application/json" \
     -d '{"text": "Direct test message"}'
   ```

3. Check AlertManager logs for errors:
   ```bash
   docker-compose logs alertmanager | grep -i slack
   ```

4. Verify environment variable is passed to container:
   ```bash
   # Docker
   docker-compose exec alertmanager env | grep SLACK

   # Kubernetes
   kubectl exec -n cryptofunk deployment/alertmanager -- env | grep SLACK
   ```

### Email Notifications Not Delivered

**Symptom**: Alerts visible but emails not received

**Check**:
1. SMTP credentials are correct
2. Firewall allows outbound port 587
3. Check spam folder
4. Test SMTP directly (see "Test Email Delivery" above)
5. Check AlertManager logs:
   ```bash
   docker-compose logs alertmanager | grep -i smtp
   ```

6. For Gmail, ensure:
   - 2FA is enabled
   - Using App Password (not regular password)
   - "Less secure app access" is NOT used (deprecated)

### Alert Rules Not Loading

**Symptom**: `curl http://localhost:9090/api/v1/rules` returns no rules

**Check**:
1. Alert rule files exist:
   ```bash
   ls -la deployments/prometheus/alerts/
   ```

2. Alert rules are valid YAML:
   ```bash
   # Install promtool
   go install github.com/prometheus/prometheus/cmd/promtool@latest

   # Validate rules
   promtool check rules deployments/prometheus/alerts/*.yml
   ```

3. Prometheus configuration loads rules:
   ```bash
   curl http://localhost:9090/api/v1/status/config | jq '.data.yaml' | grep -A5 'rule_files'
   ```

4. Restart Prometheus:
   ```bash
   docker-compose restart prometheus
   # OR
   kubectl rollout restart deployment/prometheus -n cryptofunk
   ```

### Alerts Not Firing

**Symptom**: Rules loaded but alerts never fire

**Check**:
1. Metrics exist:
   ```bash
   # Check if metric exists
   curl -G http://localhost:9090/api/v1/query \
     --data-urlencode 'query=cryptofunk_agent_status'
   ```

2. Alert expression is correct:
   ```bash
   # Test alert expression
   curl -G http://localhost:9090/api/v1/query \
     --data-urlencode 'query=cryptofunk_agent_status == 0'
   ```

3. Alert is in pending state (waiting for `for` duration):
   ```bash
   curl http://localhost:9090/api/v1/alerts | jq '.data.alerts[]? | select(.state == "pending")'
   ```

## Best Practices

1. **Start with Info alerts** - Test basic routing before critical alerts
2. **Use unique alert names** - Include timestamp in test alert names
3. **Silence test alerts** - Clean up after testing to avoid noise
4. **Test all severity levels** - Verify routing for info, warning, critical
5. **Check multiple channels** - Ensure Slack and email both work
6. **Document webhook URLs** - Store safely, never commit to git
7. **Monitor AlertManager metrics** - Track notification success/failure
8. **Regular testing** - Run test suite weekly to catch issues early

## Additional Resources

- **AlertManager Documentation**: https://prometheus.io/docs/alerting/latest/alertmanager/
- **Prometheus Alerting**: https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/
- **Alert Runbook**: `/docs/ALERT_RUNBOOK.md`
- **Slack Incoming Webhooks**: https://api.slack.com/messaging/webhooks
- **Gmail App Passwords**: https://support.google.com/accounts/answer/185833

## Quick Reference

### Useful Commands

```bash
# Check AlertManager health
curl http://localhost:9093/-/healthy

# View all alerts
curl http://localhost:9093/api/v2/alerts | jq

# View configuration
curl http://localhost:9093/api/v2/status | jq

# Send test alert
curl -X POST http://localhost:9093/api/v2/alerts -H "Content-Type: application/json" -d '[{"labels":{"alertname":"Test","severity":"info"},"annotations":{"summary":"Test"}}]'

# View alert rules
curl http://localhost:9090/api/v1/rules | jq

# Open AlertManager UI
open http://localhost:9093  # macOS
xdg-open http://localhost:9093  # Linux

# Run test suite
task test-alerts
```

### Important URLs

- **AlertManager UI**: http://localhost:9093
- **Prometheus UI**: http://localhost:9090
- **Grafana**: http://localhost:3000
- **API**: http://localhost:8080
