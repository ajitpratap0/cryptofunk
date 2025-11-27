# CryptoFunk Grafana Dashboards

Comprehensive Grafana dashboards for monitoring CryptoFunk trading system performance, health, and risk metrics.

## Dashboards Overview

### 1. Explainability & Vector Search (`explainability-dashboard.json`)

**Purpose**: Monitor LLM decision explainability and vector search performance.

**Key Metrics**:
- Vector search latency (p50, p95, p99) for semantic search and similar decisions
- Search request rate by endpoint
- Search error rate percentage
- Database connection pool metrics
- Database query latency for decision operations
- Decision API request volume
- Response time distribution for all decision endpoints
- Error breakdown by HTTP status code
- Vector search summary statistics

**Use Cases**:
- Monitoring vector search performance and reliability
- Identifying slow pgvector queries
- Tracking decision API usage patterns
- Debugging search errors and failures
- Optimizing database connection pool sizing
- Performance tuning for semantic search operations

**Recommended Alerts**:
- Vector search p95 latency > 2 seconds
- Search error rate > 5%
- Database connection pool near max capacity
- Decision API error rate > 3%

### 2. System Overview (`system-overview.json`)

**Purpose**: Monitor overall system health and infrastructure components.

**Key Metrics**:
- Component health status (orchestrator, API, MCP servers, agents)
- MCP request rate and latency by server
- Error rates across all services
- Database connection pool usage
- Redis cache hit rate
- NATS message throughput
- Component uptime

**Use Cases**:
- Real-time system health monitoring
- Performance troubleshooting
- Infrastructure capacity planning
- Identifying bottlenecks

**Recommended Alerts**:
- MCP server down for > 2 minutes
- Error rate > 5%
- Database connections near max capacity
- Cache hit rate < 50%

### 2. Trading Performance (`trading-performance.json`)

**Purpose**: Track trading performance, profitability, and execution metrics.

**Key Metrics**:
- Total P&L
- Win rate
- Open positions count
- Sharpe ratio
- Current drawdown vs threshold
- Trade frequency
- Position value by symbol
- Daily/Weekly/Monthly returns
- Winning vs losing trades value

**Use Cases**:
- Performance evaluation
- Strategy effectiveness assessment
- Risk monitoring
- Portfolio analysis

**Recommended Alerts**:
- Drawdown exceeds 20%
- Win rate drops below 40%
- Sharpe ratio < 1 for extended period
- No trades executed in 1 hour (during active trading hours)

### 3. Agent Performance (`agent-performance.json`)

**Purpose**: Monitor agent activity, health, and AI model usage.

**Key Metrics**:
- Agent health status per agent
- Signal generation rate by agent and type
- Analysis duration (p95)
- Signal confidence distribution
- LLM call rate and latency
- Token usage by agent and provider

**Use Cases**:
- Agent health monitoring
- Performance optimization
- LLM cost tracking
- Signal quality analysis

**Recommended Alerts**:
- Agent unhealthy for > 5 minutes
- Analysis duration > 10 seconds
- LLM call failure rate > 5%
- Token usage exceeds budget

### 4. Risk Metrics (`risk-metrics.json`)

**Purpose**: Monitor risk management, circuit breakers, and portfolio risk.

**Key Metrics**:
- Circuit breaker status
- Current drawdown vs threshold
- Risk limit breaches by type
- Value at Risk (VaR)
- Position sizes
- Portfolio value
- Circuit breaker trips over time
- Sharpe ratio trend

**Use Cases**:
- Risk monitoring and control
- Circuit breaker tracking
- Portfolio risk assessment
- Compliance verification

**Recommended Alerts**:
- Circuit breaker tripped
- Drawdown > 15%
- Risk limit breach
- VaR exceeds threshold

## Installation

### Method 1: Docker Compose (Recommended for Development)

1. **Update docker-compose.yml** to include Grafana:

```yaml
services:
  grafana:
    image: grafana/grafana:latest
    container_name: cryptofunk-grafana
    ports:
      - "3000:3000"
    environment:
      - GF_SECURITY_ADMIN_USER=admin
      - GF_SECURITY_ADMIN_PASSWORD=admin
      - GF_INSTALL_PLUGINS=
    volumes:
      - grafana-data:/var/lib/grafana
      - ./deployments/grafana/dashboards:/var/lib/grafana/dashboards/cryptofunk:ro
      - ./deployments/grafana/provisioning.yml:/etc/grafana/provisioning/dashboards/cryptofunk.yml:ro
    depends_on:
      - prometheus
    networks:
      - cryptofunk

volumes:
  grafana-data:
```

2. **Start Grafana**:

```bash
docker-compose up -d grafana
```

3. **Access Grafana**:
- URL: http://localhost:3000
- Default credentials: admin/admin
- Change password on first login

4. **Add Prometheus Data Source**:
- Go to Configuration → Data Sources
- Add new Prometheus data source
- URL: http://prometheus:9090
- Save & Test

5. **Dashboards are auto-loaded** from the provisioning configuration.

### Method 2: Kubernetes (Recommended for Production)

1. **Install Grafana using Helm**:

```bash
helm repo add grafana https://grafana.github.io/helm-charts
helm repo update

helm install grafana grafana/grafana \
  --namespace cryptofunk \
  --create-namespace \
  --set persistence.enabled=true \
  --set persistence.size=10Gi \
  --set adminPassword=admin
```

2. **Create ConfigMap with dashboards**:

```bash
kubectl create configmap cryptofunk-dashboards \
  --from-file=deployments/grafana/dashboards/ \
  --namespace=cryptofunk
```

3. **Mount ConfigMap in Grafana deployment**:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-dashboard-provider
  namespace: cryptofunk
data:
  provider.yaml: |
    apiVersion: 1
    providers:
      - name: 'CryptoFunk'
        orgId: 1
        folder: 'CryptoFunk'
        type: file
        disableDeletion: false
        options:
          path: /var/lib/grafana/dashboards/cryptofunk
---
# Add volume mount to Grafana deployment
volumes:
  - name: dashboards
    configMap:
      name: cryptofunk-dashboards
  - name: dashboard-provider
    configMap:
      name: grafana-dashboard-provider
volumeMounts:
  - name: dashboards
    mountPath: /var/lib/grafana/dashboards/cryptofunk
  - name: dashboard-provider
    mountPath: /etc/grafana/provisioning/dashboards
```

4. **Access Grafana**:

```bash
# Port forward to access locally
kubectl port-forward svc/grafana 3000:80 -n cryptofunk

# Or use LoadBalancer/Ingress in production
```

### Method 3: Manual Import

1. **Access Grafana** UI
2. Go to **Dashboards → Import**
3. Upload each JSON file from `deployments/grafana/dashboards/`
4. Select Prometheus data source
5. Import

## Configuration

### Prometheus Data Source

Ensure Prometheus is configured as a data source:

```
Name: Prometheus
Type: Prometheus
URL: http://prometheus:9090 (or your Prometheus URL)
Access: Server (default)
```

### Dashboard Variables

Some dashboards support variables for filtering:

- **Environment**: Filter by environment (dev, staging, prod)
- **Agent**: Filter by specific agent
- **Symbol**: Filter by trading symbol
- **Mode**: Filter by trading mode (paper, live)

To add variables:
1. Dashboard Settings → Variables
2. Add new variable
3. Query: `label_values(cryptofunk_agent_healthy, agent)`

### Time Ranges

Default time ranges:
- System Overview: Last 1 hour
- Trading Performance: Last 6 hours
- Agent Performance: Last 1 hour
- Risk Metrics: Last 1 hour

Adjust via dashboard time picker (top right).

## Customization

### Adding Custom Panels

1. Edit dashboard in Grafana UI
2. Add panel
3. Select visualization type
4. Configure PromQL query
5. Save dashboard
6. Export JSON and save to repository

### PromQL Query Examples

**Request rate by server**:
```promql
sum(rate(cryptofunk_mcp_requests_total[5m])) by (server)
```

**P95 latency**:
```promql
histogram_quantile(0.95, rate(cryptofunk_mcp_request_duration_seconds_bucket[5m]))
```

**Error rate**:
```promql
rate(cryptofunk_mcp_requests_total{status="error"}[5m]) /
rate(cryptofunk_mcp_requests_total[5m])
```

**Agent signals per minute**:
```promql
rate(cryptofunk_agent_signals_total[1m]) * 60
```

### Alert Rules

Configure alerts in Grafana or Prometheus AlertManager:

1. **In Grafana**: Dashboard panel → Alert tab → Create alert
2. **In Prometheus**: Add alerting rules to `prometheus-alerts.yml`

Example alert rules are documented in `docs/METRICS_INTEGRATION.md`.

## Troubleshooting

### Dashboards Not Showing Data

1. **Check Prometheus connection**:
   - Grafana → Configuration → Data Sources → Prometheus → Test
   - Should return "Data source is working"

2. **Verify metrics are being scraped**:
   - Go to Prometheus UI: http://localhost:9090
   - Status → Targets
   - All targets should be "UP"

3. **Check metrics exist**:
   - Prometheus → Graph → Enter metric name
   - Example: `cryptofunk_mcp_requests_total`

4. **Verify time range**:
   - Ensure dashboard time range includes period when system was running
   - Try "Last 24 hours" to see historical data

### Missing Metrics

If specific metrics are missing:

1. **Check component is running**:
   ```bash
   curl http://localhost:9201/metrics  # MCP server
   curl http://localhost:9101/metrics  # Agent
   ```

2. **Verify Prometheus scrape config**:
   - Check `deployments/prometheus/prometheus.yml`
   - Ensure target is listed in appropriate job

3. **Check component logs**:
   ```bash
   docker-compose logs -f market-data-server
   ```

### Performance Issues

If dashboards are slow:

1. **Reduce query time range**: Use shorter time windows
2. **Increase scrape interval**: Edit Prometheus config
3. **Add query rate limiting**: Configure in Grafana settings
4. **Use recording rules**: Pre-calculate expensive queries in Prometheus

## Best Practices

### Dashboard Organization

- **System Overview**: Primary dashboard, always visible
- **Trading Performance**: Review daily
- **Agent Performance**: Check when debugging issues
- **Risk Metrics**: Monitor continuously during live trading

### Refresh Rates

- Development: 10s refresh
- Staging: 30s refresh
- Production: 10s refresh for critical dashboards, 1m for others

### Alerting Strategy

1. **Critical alerts** → PagerDuty/phone
   - Circuit breaker tripped
   - System down
   - High drawdown

2. **Warning alerts** → Slack/email
   - High error rate
   - Performance degradation
   - Risk limit approached

3. **Info alerts** → Dashboard annotations
   - Deployment events
   - Configuration changes

### Data Retention

Configure in Grafana:
- Short-term (1 week): 1-minute resolution
- Medium-term (1 month): 5-minute resolution
- Long-term (1 year): 1-hour resolution

## Integration with Alerting

### Slack Integration

1. Configure Slack webhook in Grafana
2. Add notification channel: Configuration → Notification channels
3. Test notification
4. Add alerts to dashboard panels

### Email Notifications

1. Configure SMTP in Grafana config:
```ini
[smtp]
enabled = true
host = smtp.gmail.com:587
user = alerts@cryptofunk.com
password = ***
from_address = alerts@cryptofunk.com
```

2. Create email notification channel
3. Assign to alerts

### PagerDuty Integration

1. Install PagerDuty plugin:
   ```bash
   grafana-cli plugins install pagerduty
   ```

2. Configure PagerDuty notification channel with integration key
3. Route critical alerts to PagerDuty

## Maintenance

### Backup Dashboards

Regularly export and commit dashboard JSON:

```bash
# Export from Grafana UI
Dashboard → Share → Export → Save to file → Copy JSON

# Or use Grafana API
curl -H "Authorization: Bearer $GRAFANA_API_KEY" \
  http://localhost:3000/api/dashboards/uid/cryptofunk-system > system-overview.json
```

### Update Dashboards

1. Make changes in Grafana UI
2. Test thoroughly
3. Export JSON
4. Commit to repository
5. Deploy to other environments

### Version Control

Dashboard JSON files are version controlled in this repository:
- `deployments/grafana/dashboards/*.json`
- Commit changes with descriptive messages
- Use branches for experimental dashboards

## Resources

- Grafana Documentation: https://grafana.com/docs/
- Prometheus Queries: https://prometheus.io/docs/prometheus/latest/querying/basics/
- Dashboard Best Practices: https://grafana.com/docs/grafana/latest/best-practices/
- CryptoFunk Metrics: `docs/METRICS_INTEGRATION.md`

## Support

For issues with dashboards:
1. Check Grafana logs: `docker-compose logs grafana`
2. Review Prometheus targets: http://localhost:9090/targets
3. Consult `docs/METRICS_INTEGRATION.md`
4. Contact: ops@cryptofunk.com
