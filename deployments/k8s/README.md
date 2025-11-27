# Kubernetes Deployment Guide

This directory contains Kubernetes manifests for deploying CryptoFunk to production Kubernetes clusters.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Architecture](#architecture)
- [Directory Structure](#directory-structure)
- [Configuration](#configuration)
- [Deployment Steps](#deployment-steps)
- [Scaling](#scaling)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)
- [Security](#security)
- [Maintenance](#maintenance)

## Prerequisites

### Required Tools

- **kubectl** 1.28+
- **kustomize** 5.0+ (or use kubectl built-in kustomize)
- **Kubernetes cluster** 1.28+ with:
  - At least 3 worker nodes (recommended)
  - 16 CPU cores total (minimum)
  - 32GB RAM total (minimum)
  - 200GB storage available

### Required Kubernetes Components

1. **Ingress Controller** (choose one):
   - NGINX Ingress Controller (recommended)
   - Traefik
   - HAProxy Ingress

2. **Storage Provisioner**:
   - Dynamic PV provisioning (e.g., AWS EBS, GCE PD, Azure Disk)
   - Or configure static PVs

3. **Optional but Recommended**:
   - cert-manager (for TLS certificates)
   - metrics-server (for HPA)
   - Prometheus Operator (enhanced monitoring)

### Installation Commands

```bash
# NGINX Ingress Controller
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.1/deploy/static/provider/cloud/deploy.yaml

# cert-manager (for TLS)
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml

# metrics-server (for autoscaling)
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

## Quick Start

### 1. Build and Push Docker Images

```bash
# Build all images
cd ../..  # Back to project root
docker-compose build

# Tag images for your registry
docker tag cryptofunk/orchestrator:latest your-registry.io/cryptofunk/orchestrator:v1.0.0
docker tag cryptofunk/api:latest your-registry.io/cryptofunk/api:v1.0.0
# ... tag all images

# Push to registry
docker push your-registry.io/cryptofunk/orchestrator:v1.0.0
docker push your-registry.io/cryptofunk/api:v1.0.0
# ... push all images
```

### 2. Configure Secrets

```bash
# Create secrets from environment file
cd deployments/k8s/base

# Edit secrets.yaml with base64-encoded values
echo -n "your_postgres_password" | base64
echo -n "your_anthropic_api_key" | base64
# ... encode all secrets

# Update secrets.yaml with encoded values
vim secrets.yaml
```

### 3. Update Configuration

```bash
# Edit configmap.yaml for your environment
vim configmap.yaml

# Edit kustomization.yaml to point to your registry
vim kustomization.yaml

# Edit ingress.yaml with your domain names
vim ingress.yaml
```

### 4. Deploy to Kubernetes

```bash
# Apply all manifests
kubectl apply -k .

# Or use kustomize separately
kustomize build . | kubectl apply -f -

# Check deployment status
kubectl get pods -n cryptofunk
kubectl get svc -n cryptofunk
kubectl get ingress -n cryptofunk
```

## Architecture

### Kubernetes Resources

```
cryptofunk namespace
│
├── Infrastructure Layer
│   ├── postgres (Deployment + PVC + Service)
│   ├── redis (Deployment + PVC + Service)
│   ├── nats (Deployment + Service)
│   ├── bifrost (Deployment + Service)
│   ├── prometheus (Deployment + PVC + Service)
│   └── grafana (Deployment + PVC + Service)
│
├── Application Layer
│   ├── migrate (Job - one-shot)
│   ├── orchestrator (Deployment + Service)
│   ├── MCP Servers (4 Deployments)
│   │   ├── market-data-server
│   │   ├── technical-indicators-server
│   │   ├── risk-analyzer-server
│   │   └── order-executor-server
│   ├── Trading Agents (6 Deployments)
│   │   ├── technical-agent
│   │   ├── orderbook-agent
│   │   ├── sentiment-agent
│   │   ├── trend-agent
│   │   ├── reversion-agent
│   │   └── risk-agent
│   └── api (Deployment + Service + Ingress)
│
└── Configuration
    ├── ConfigMaps (app config)
    ├── Secrets (API keys, passwords)
    ├── PVCs (persistent storage)
    └── Ingress (external routing)
```

### Resource Requirements

| Component | Replicas | CPU Request | CPU Limit | Memory Request | Memory Limit |
|-----------|----------|-------------|-----------|----------------|--------------|
| postgres | 1 | 500m | 2000m | 1Gi | 4Gi |
| redis | 1 | 100m | 500m | 512Mi | 1Gi |
| nats | 1 | 100m | 500m | 256Mi | 512Mi |
| bifrost | 2 | 200m | 1000m | 512Mi | 2Gi |
| prometheus | 1 | 200m | 1000m | 1Gi | 2Gi |
| grafana | 1 | 100m | 500m | 256Mi | 1Gi |
| orchestrator | 1 | 200m | 1000m | 512Mi | 2Gi |
| MCP servers | 2 each | 100m | 500m | 256Mi | 512Mi |
| Agents | 2 each | 100m | 500m | 256Mi | 512Mi |
| API | 3 | 200m | 1000m | 512Mi | 1Gi |

**Total Minimum**: ~8 CPU cores, ~16GB RAM (requests)
**Total Maximum**: ~32 CPU cores, ~48GB RAM (limits)

## Directory Structure

```
k8s/
├── base/                       # Base manifests (environment-agnostic)
│   ├── namespace.yaml          # cryptofunk namespace
│   ├── configmap.yaml          # Application configuration
│   ├── secrets.yaml            # Secret values (template)
│   ├── pvc.yaml                # PersistentVolumeClaims
│   ├── deployment-*.yaml       # Deployment manifests
│   ├── job-migrate.yaml        # Database migration job
│   ├── services.yaml           # Service definitions
│   ├── ingress.yaml            # Ingress routing
│   └── kustomization.yaml      # Kustomize base
│
├── overlays/                   # Environment-specific overlays
│   ├── dev/                    # Development environment
│   │   └── kustomization.yaml
│   ├── staging/                # Staging environment
│   │   └── kustomization.yaml
│   └── prod/                   # Production environment
│       └── kustomization.yaml
│
└── README.md                   # This file
```

## Configuration

### ConfigMaps

Edit `base/configmap.yaml` for application configuration:

```yaml
data:
  TRADING_MODE: "PAPER"    # PAPER or LIVE
  LOG_LEVEL: "info"        # debug, info, warn, error
  CORS_ORIGINS: "*"        # Comma-separated domains or *
```

### Secrets

Edit `base/secrets.yaml` with base64-encoded values:

```bash
# Encode secrets
echo -n "your_postgres_password" | base64
echo -n "sk-ant-api03-your-key" | base64

# Update secrets.yaml
vim base/secrets.yaml
```

**IMPORTANT**: Never commit actual secrets to version control!

### Image Registry

Edit `base/kustomization.yaml` to use your registry:

```yaml
images:
  - name: cryptofunk/orchestrator
    newName: your-registry.io/cryptofunk/orchestrator
    newTag: v1.0.0
```

### Domains

Edit `base/ingress.yaml` with your domains:

```yaml
rules:
  - host: api.cryptofunk.example.com     # Your API domain
  - host: grafana.cryptofunk.example.com # Your Grafana domain
```

## Deployment Steps

### Step 1: Create Namespace

```bash
kubectl apply -f base/namespace.yaml
```

### Step 2: Create Secrets

```bash
# IMPORTANT: Update secrets.yaml first!
kubectl apply -f base/secrets.yaml

# Verify secrets created
kubectl get secrets -n cryptofunk
```

### Step 3: Create ConfigMaps

```bash
kubectl apply -f base/configmap.yaml
```

### Step 4: Create PVCs

```bash
kubectl apply -f base/pvc.yaml

# Wait for PVCs to be bound
kubectl get pvc -n cryptofunk -w
```

### Step 5: Deploy Infrastructure

```bash
# Deploy in order:
kubectl apply -f base/deployment-postgres.yaml
kubectl apply -f base/deployment-redis.yaml
kubectl apply -f base/deployment-nats.yaml
kubectl apply -f base/deployment-bifrost.yaml
kubectl apply -f base/deployment-prometheus.yaml
kubectl apply -f base/deployment-alertmanager.yaml
kubectl apply -f base/deployment-grafana.yaml

# Wait for infrastructure pods
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=database -n cryptofunk --timeout=300s
kubectl wait --for=condition=ready pod -l app.kubernetes.io/component=cache -n cryptofunk --timeout=300s
```

### Step 6: Run Database Migration

```bash
kubectl apply -f base/job-migrate.yaml

# Check migration status
kubectl logs -f job/migrate -n cryptofunk

# Verify migration completed
kubectl get jobs -n cryptofunk
```

### Step 7: Deploy Application Services

```bash
# Deploy orchestrator first
kubectl apply -f base/deployment-orchestrator.yaml

# Wait for orchestrator
kubectl wait --for=condition=ready pod -l app.kubernetes.io/name=orchestrator -n cryptofunk --timeout=120s

# Deploy MCP servers and agents
kubectl apply -f base/deployment-mcp-servers.yaml
kubectl apply -f base/deployment-agents.yaml

# Deploy API
kubectl apply -f base/deployment-api.yaml
```

### Step 8: Create Services

```bash
kubectl apply -f base/services.yaml

# Check services
kubectl get svc -n cryptofunk
```

### Step 9: Create Ingress

```bash
kubectl apply -f base/ingress.yaml

# Get ingress IP
kubectl get ingress -n cryptofunk
```

### Step 10: Verify Deployment

```bash
# Check all pods are running
kubectl get pods -n cryptofunk

# Check pod logs
kubectl logs -f deployment/orchestrator -n cryptofunk
kubectl logs -f deployment/api -n cryptofunk

# Test API health
curl http://api.cryptofunk.example.com/health
```

## Scaling

### Horizontal Pod Autoscaling (HPA)

```bash
# Scale API based on CPU
kubectl autoscale deployment api -n cryptofunk --cpu-percent=70 --min=3 --max=10

# Scale agents based on CPU
kubectl autoscale deployment technical-agent -n cryptofunk --cpu-percent=70 --min=2 --max=5

# Check HPA status
kubectl get hpa -n cryptofunk
```

### Manual Scaling

```bash
# Scale specific deployment
kubectl scale deployment api -n cryptofunk --replicas=5
kubectl scale deployment technical-agent -n cryptofunk --replicas=3

# Scale multiple deployments
kubectl scale deployment market-data-server technical-indicators-server -n cryptofunk --replicas=3
```

## Monitoring

### Access Dashboards

```bash
# Get service URLs
kubectl get svc -n cryptofunk

# Port-forward to access locally
kubectl port-forward svc/grafana-service 3000:3000 -n cryptofunk
kubectl port-forward svc/prometheus-service 9090:9090 -n cryptofunk

# Access:
# Grafana: http://localhost:3000
# Prometheus: http://localhost:9090
```

### Check Pod Status

```bash
# All pods
kubectl get pods -n cryptofunk

# Specific deployment
kubectl get pods -l app.kubernetes.io/name=orchestrator -n cryptofunk

# Pod details
kubectl describe pod <pod-name> -n cryptofunk

# Pod logs
kubectl logs -f <pod-name> -n cryptofunk

# Previous logs (if pod crashed)
kubectl logs --previous <pod-name> -n cryptofunk
```

### Resource Usage

```bash
# Node resources
kubectl top nodes

# Pod resources
kubectl top pods -n cryptofunk

# Check PVC usage
kubectl get pvc -n cryptofunk
kubectl describe pvc postgres-pvc -n cryptofunk
```

## Troubleshooting

### Pods Not Starting

```bash
# Check pod events
kubectl describe pod <pod-name> -n cryptofunk

# Common issues:
# 1. Image pull errors - check registry credentials
# 2. PVC binding issues - check storage class
# 3. Resource limits - check node capacity
```

### Database Connection Issues

```bash
# Check postgres pod
kubectl logs -f deployment/postgres -n cryptofunk

# Test connection from another pod
kubectl run -it --rm debug --image=postgres:17 --restart=Never -n cryptofunk -- psql -h postgres-service -U postgres -d cryptofunk

# Check secret values
kubectl get secret cryptofunk-secrets -n cryptofunk -o yaml
```

### Service Discovery Issues

```bash
# Check services
kubectl get svc -n cryptofunk

# Check endpoints
kubectl get endpoints -n cryptofunk

# DNS test
kubectl run -it --rm debug --image=busybox --restart=Never -n cryptofunk -- nslookup postgres-service
```

### Ingress Not Working

```bash
# Check ingress status
kubectl describe ingress cryptofunk-ingress -n cryptofunk

# Check ingress controller
kubectl get pods -n ingress-nginx

# Check ingress controller logs
kubectl logs -f deployment/ingress-nginx-controller -n ingress-nginx
```

## Security

### TLS/HTTPS Setup

1. **Install cert-manager**:
```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
```

2. **Create ClusterIssuer**:
```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: your-email@example.com
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
    - http01:
        ingress:
          class: nginx
```

3. **Update Ingress**:
```yaml
metadata:
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
spec:
  tls:
  - hosts:
    - api.cryptofunk.example.com
    secretName: cryptofunk-tls
```

### Network Policies

Create network policies to restrict traffic:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: postgres-netpol
  namespace: cryptofunk
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: postgres
  policyTypes:
  - Ingress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app.kubernetes.io/part-of: cryptofunk-trading-system
```

### RBAC

Configure Role-Based Access Control:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: cryptofunk-viewer
  namespace: cryptofunk
rules:
- apiGroups: [""]
  resources: ["pods", "services"]
  verbs: ["get", "list", "watch"]
```

## Maintenance

### Backup PostgreSQL

```bash
# Create backup job
kubectl create job --from=cronjob/postgres-backup postgres-backup-manual -n cryptofunk

# Or manual backup
kubectl exec -it deployment/postgres -n cryptofunk -- pg_dump -U postgres cryptofunk > backup.sql
```

### Update Images

```bash
# Update image tag in kustomization.yaml
# Then apply
kubectl apply -k base/

# Or use kubectl set image
kubectl set image deployment/api api=your-registry.io/cryptofunk/api:v1.1.0 -n cryptofunk

# Rolling update status
kubectl rollout status deployment/api -n cryptofunk

# Rollback if needed
kubectl rollout undo deployment/api -n cryptofunk
```

### Clean Up

```bash
# Delete all resources
kubectl delete -k base/

# Or delete namespace (removes everything)
kubectl delete namespace cryptofunk

# Delete PVCs (WARNING: deletes data!)
kubectl delete pvc --all -n cryptofunk
```

## Production Checklist

- [ ] Update all secrets in `secrets.yaml` with strong values
- [ ] Configure image registry in `kustomization.yaml`
- [ ] Set appropriate resource requests/limits
- [ ] Configure ingress with your domains
- [ ] Set up TLS/HTTPS with cert-manager
- [ ] Configure network policies for security
- [ ] Set up RBAC for access control
- [ ] Configure PostgreSQL backups
- [ ] Set up monitoring alerts in Prometheus
- [ ] Configure log aggregation (ELK, Loki, etc.)
- [ ] Test disaster recovery procedures
- [ ] Document runbooks for operations
- [ ] Set `TRADING_MODE=PAPER` initially, then `LIVE` after testing

## Further Reading

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [Kustomize Documentation](https://kustomize.io/)
- [NGINX Ingress Controller](https://kubernetes.github.io/ingress-nginx/)
- [cert-manager Documentation](https://cert-manager.io/docs/)
- [CryptoFunk Architecture](../../docs/ARCHITECTURE.md)
