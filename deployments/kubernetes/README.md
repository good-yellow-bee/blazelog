# BlazeLog Kubernetes Deployment

Deploy BlazeLog to Kubernetes with a single command.

## Quick Start

```bash
# 1. Create namespace and secrets
kubectl create namespace blazelog

# Generate and create secrets
MASTER_KEY=$(openssl rand -base64 32)
JWT_SECRET=$(openssl rand -base64 32)
CSRF_SECRET=$(openssl rand -base64 32)
CLICKHOUSE_PASSWORD=$(openssl rand -base64 16)

kubectl create secret generic blazelog-secrets \
  --namespace=blazelog \
  --from-literal=master-key="$MASTER_KEY" \
  --from-literal=jwt-secret="$JWT_SECRET" \
  --from-literal=csrf-secret="$CSRF_SECRET" \
  --from-literal=clickhouse-password="$CLICKHOUSE_PASSWORD"

# 2. Deploy (choose one)
kubectl apply -k deployments/kubernetes/base              # Basic
kubectl apply -k deployments/kubernetes/overlays/dev      # Development
kubectl apply -k deployments/kubernetes/overlays/prod     # Production

# 3. Access UI
kubectl port-forward -n blazelog svc/blazelog-server 8080:8080
# Open http://localhost:8080
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         KUBERNETES CLUSTER                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌─────────────┐     ┌─────────────┐     ┌─────────────┐        │
│  │   Node 1    │     │   Node 2    │     │   Node 3    │        │
│  │ ┌─────────┐ │     │ ┌─────────┐ │     │ ┌─────────┐ │        │
│  │ │ Agent   │ │     │ │ Agent   │ │     │ │ Agent   │ │        │
│  │ │DaemonSet│ │     │ │DaemonSet│ │     │ │DaemonSet│ │        │
│  │ └────┬────┘ │     │ └────┬────┘ │     │ └────┬────┘ │        │
│  └──────┼──────┘     └──────┼──────┘     └──────┼──────┘        │
│         │                   │                   │                │
│         └───────────────────┼───────────────────┘                │
│                             │ gRPC (9443)                        │
│                             ▼                                    │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                    blazelog-server                        │   │
│  │  ┌────────────────┐  ┌────────────────┐                  │   │
│  │  │   Deployment   │  │    Service     │                  │   │
│  │  │   (1 replica)  │  │  (ClusterIP)   │                  │   │
│  │  └───────┬────────┘  └────────────────┘                  │   │
│  │          │                                                │   │
│  │          │ Port 9000                                      │   │
│  │          ▼                                                │   │
│  │  ┌────────────────┐  ┌────────────────┐                  │   │
│  │  │   ClickHouse   │  │      PVC       │                  │   │
│  │  │  StatefulSet   │  │   (100Gi)      │                  │   │
│  │  └────────────────┘  └────────────────┘                  │   │
│  └──────────────────────────────────────────────────────────┘   │
│                             │                                    │
│                             │ HTTP (8080)                        │
│                             ▼                                    │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                       Ingress                             │   │
│  │              blazelog.example.com                         │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
deployments/kubernetes/
├── base/                      # Base manifests
│   ├── kustomization.yaml     # Kustomize config
│   ├── namespace.yaml         # Namespace definition
│   ├── secrets.yaml           # Secret templates
│   ├── pvc.yaml               # PersistentVolumeClaims
│   ├── server-configmap.yaml  # Server configuration
│   ├── server-deployment.yaml # Server Deployment
│   ├── server-service.yaml    # Server Services
│   ├── agent-configmap.yaml   # Agent configuration
│   ├── agent-daemonset.yaml   # Agent DaemonSet
│   ├── clickhouse-statefulset.yaml  # ClickHouse StatefulSet
│   ├── clickhouse-service.yaml      # ClickHouse Services
│   ├── ingress.yaml           # Ingress rules
│   └── networkpolicy.yaml     # NetworkPolicies
├── overlays/
│   ├── dev/                   # Development overlay
│   │   └── kustomization.yaml
│   └── prod/                  # Production overlay
│       ├── kustomization.yaml
│       └── hpa.yaml           # HorizontalPodAutoscaler
└── README.md
```

## Components

### Server Deployment
- REST API on port 8080
- gRPC endpoint on port 9443
- Prometheus metrics on port 9090
- Runs as non-root user (65532)
- ReadWriteOnce PVC for SQLite

### Agent DaemonSet
- Runs on every node (configurable via nodeSelector)
- Collects logs from host directories
- RBAC for pod/node metadata enrichment
- Tolerates all taints by default

### ClickHouse StatefulSet
- Log storage for production
- 100Gi PVC per replica
- Native (9000) and HTTP (8123) ports
- Optional: use external ClickHouse instead

### Networking
- **Ingress**: Nginx ingress for Web UI
- **NetworkPolicy**: Restricts traffic between components
- **Services**: ClusterIP for internal, LoadBalancer optional

## Configuration

### Secrets

**Required secrets:**
```bash
kubectl create secret generic blazelog-secrets \
  --namespace=blazelog \
  --from-literal=master-key="$(openssl rand -base64 32)" \
  --from-literal=jwt-secret="$(openssl rand -base64 32)" \
  --from-literal=csrf-secret="$(openssl rand -base64 32)" \
  --from-literal=clickhouse-password="$(openssl rand -base64 16)"
```

**TLS for mTLS (optional):**
```bash
kubectl create secret tls blazelog-tls \
  --namespace=blazelog \
  --cert=server.crt \
  --key=server.key

kubectl create secret generic blazelog-ca \
  --namespace=blazelog \
  --from-file=ca.crt
```

### ConfigMaps

Server configuration in `server-configmap.yaml`. Key settings:

| Setting | Description | Default |
|---------|-------------|---------|
| `clickhouse.enabled` | Enable ClickHouse storage | `false` |
| `tls.enabled` | Enable mTLS for agents | `false` |
| `auth.access_token_ttl` | JWT access token TTL | `15m` |
| `auth.refresh_token_ttl` | JWT refresh token TTL | `168h` |

### Ingress

Update `ingress.yaml` with your domain:
```yaml
spec:
  rules:
    - host: your-domain.com
```

For TLS with cert-manager:
```yaml
metadata:
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
```

## Deployment Options

### Development (SQLite only)
```bash
kubectl apply -k deployments/kubernetes/overlays/dev
```
- Reduced resource limits
- SQLite storage
- Suitable for testing

### Production (ClickHouse)
```bash
kubectl apply -k deployments/kubernetes/overlays/prod
```
- Full resource allocation
- ClickHouse for log storage
- HPA enabled
- Network policies enforced

### External ClickHouse

To use external ClickHouse instead of StatefulSet:

1. Remove ClickHouse from kustomization:
```yaml
resources:
  # - clickhouse-statefulset.yaml
  # - clickhouse-service.yaml
```

2. Update server ConfigMap:
```yaml
clickhouse:
  enabled: true
  addresses:
    - "external-clickhouse.example.com:9000"
```

## Operations

### Scaling

**Note:** Server requires external database for horizontal scaling (SQLite is single-node).

```bash
# View current replicas
kubectl get deploy -n blazelog

# Scale (only with external DB)
kubectl scale deploy blazelog-server -n blazelog --replicas=3
```

### Monitoring

Prometheus metrics available at `:9090/metrics`:
```bash
kubectl port-forward -n blazelog svc/blazelog-server 9090:9090
curl http://localhost:9090/metrics
```

### Logs

```bash
# Server logs
kubectl logs -n blazelog -l app.kubernetes.io/component=server -f

# Agent logs (all nodes)
kubectl logs -n blazelog -l app.kubernetes.io/component=agent -f

# ClickHouse logs
kubectl logs -n blazelog -l app.kubernetes.io/component=clickhouse -f
```

### Health Checks

```bash
# Server health
kubectl exec -n blazelog deploy/blazelog-server -- \
  wget -qO- http://localhost:8080/health/ready

# ClickHouse health
kubectl exec -n blazelog sts/blazelog-clickhouse -- \
  clickhouse-client --query "SELECT 1"
```

### Backup

**SQLite (data PVC):**
```bash
kubectl exec -n blazelog deploy/blazelog-server -- \
  sqlite3 /data/blazelog.db ".backup /tmp/backup.db"
kubectl cp blazelog/$(kubectl get pod -n blazelog -l app.kubernetes.io/component=server -o jsonpath='{.items[0].metadata.name}'):/tmp/backup.db ./blazelog-backup.db
```

**ClickHouse:**
```bash
kubectl exec -n blazelog sts/blazelog-clickhouse-0 -- \
  clickhouse-client --query "BACKUP DATABASE blazelog TO Disk('backups', 'blazelog')"
```

## Troubleshooting

### Pod not starting

```bash
# Check events
kubectl describe pod -n blazelog <pod-name>

# Check logs
kubectl logs -n blazelog <pod-name> --previous
```

### Network connectivity issues

```bash
# Test server→clickhouse
kubectl exec -n blazelog deploy/blazelog-server -- \
  nc -zv blazelog-clickhouse 9000

# Test agent→server
kubectl exec -n blazelog ds/blazelog-agent -- \
  nc -zv blazelog-server 9443
```

### Storage issues

```bash
# Check PVC status
kubectl get pvc -n blazelog

# Check PV binding
kubectl describe pvc -n blazelog blazelog-data
```

## Cleanup

```bash
# Delete all resources
kubectl delete -k deployments/kubernetes/base

# Delete namespace (removes everything)
kubectl delete namespace blazelog

# Delete PVCs separately (not deleted with namespace)
kubectl delete pvc -n blazelog --all
```

## Requirements

- Kubernetes 1.25+
- kubectl configured
- Ingress controller (nginx-ingress recommended)
- CNI with NetworkPolicy support (Calico, Cilium)
- Storage class for PVCs
- Optional: cert-manager for TLS
