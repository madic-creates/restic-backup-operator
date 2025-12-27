# Installation and Deployment

## Prerequisites

- Kubernetes 1.26+
- Helm 3.x (for Helm installation)
- Kustomize (for Kustomize installation)
- Prometheus + Pushgateway (optional, for metrics)

## Installation Methods

### Helm (OCI Registry)

```bash
# Install the operator directly from OCI registry
helm install restic-backup-operator oci://ghcr.io/madic-creates/charts/restic-backup-operator \
  --namespace backup-system \
  --create-namespace
```

To install a specific version:

```bash
helm install restic-backup-operator oci://ghcr.io/madic-creates/charts/restic-backup-operator \
  --version 0.1.0 \
  --namespace backup-system \
  --create-namespace
```

#### Helm Values

Example `values.yaml`:

```yaml
replicaCount: 1

image:
  repository: ghcr.io/madic-creates/restic-backup-operator
  tag: latest
  pullPolicy: IfNotPresent

resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 256Mi

serviceAccount:
  create: true
  name: restic-backup-operator

# Leader election for HA
leaderElection:
  enabled: true

# Stale lock threshold - duration after which repository locks are
# considered stale and can be automatically removed.
# This prevents the operator from interfering with active backup operations.
# Format: Go duration string (e.g., "30m", "1h", "2h30m")
staleLockThreshold: "30m"

# Default restic image for backup jobs
defaultResticImage: ghcr.io/restic/restic:0.18.1
```

Install with custom values:

```bash
helm install restic-backup-operator oci://ghcr.io/madic-creates/charts/restic-backup-operator \
  --namespace backup-system \
  --create-namespace \
  -f values.yaml
```

### Kustomize

Create a `kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: backup-system

resources:
  - https://github.com/madic-creates/restic-backup-operator/config/default?ref=v0.0.3

# Optional: patch resources
patches:
  - patch: |-
      - op: replace
        path: /spec/template/spec/containers/0/resources/limits/memory
        value: 512Mi
    target:
      kind: Deployment
      name: restic-backup-operator-controller-manager
```

Apply:

```bash
kubectl create namespace backup-system
kubectl apply -k .
```

### Kustomize with Helm (OCI)

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: backup-system

helmCharts:
  - name: restic-backup-operator
    repo: oci://ghcr.io/madic-creates/charts
    version: 0.1.0
    releaseName: restic-backup-operator
    namespace: backup-system
    valuesFile: values.yaml
```

## Verification

Check the operator is running:

```bash
kubectl get pods -n backup-system
kubectl get crds | grep resticbackup
```

Expected CRDs:
- `resticbackups.backup.resticbackup.io`
- `resticrepositories.backup.resticbackup.io`
- `resticrestores.backup.resticbackup.io`
- `globalretentionpolicies.backup.resticbackup.io`

## Quick Start

After installation, create a repository and backup:

```yaml
# 1. Create repository credentials
apiVersion: v1
kind: Secret
metadata:
  name: restic-credentials
  namespace: backup-system
type: Opaque
stringData:
  RESTIC_PASSWORD: "your-restic-password"
  AWS_ACCESS_KEY_ID: "your-access-key"
  AWS_SECRET_ACCESS_KEY: "your-secret-key"
---
# 2. Create ResticRepository
apiVersion: backup.resticbackup.io/v1alpha1
kind: ResticRepository
metadata:
  name: my-repository
  namespace: backup-system
spec:
  repositoryURL: s3:s3.amazonaws.com/my-backup-bucket
  credentialsSecretRef:
    name: restic-credentials
---
# 3. Create ResticBackup
apiVersion: backup.resticbackup.io/v1alpha1
kind: ResticBackup
metadata:
  name: my-app-backup
  namespace: my-app
spec:
  repositoryRef:
    name: my-repository
    namespace: backup-system
  schedule: "0 2 * * *"
  source:
    pvc:
      claimName: my-app-data
      paths:
        - /data
  retention:
    keepDaily: 7
    keepWeekly: 4
    keepMonthly: 6
```

## Operator Configuration

### Stale Lock Handling

Restic uses locks to prevent concurrent access to repositories. If a backup operation is interrupted (e.g., pod crash, node failure), stale locks may remain and block subsequent operations.

The operator automatically detects and removes stale locks based on a configurable threshold:

| Configuration | Description |
|---------------|-------------|
| `staleLockThreshold` | Duration after which a lock is considered stale. Default: `30m` |

**How it works:**

1. When the operator detects a locked repository, it checks the lock age
2. If the lock is **older** than the threshold, it's removed automatically
3. If the lock is **newer** than the threshold, the operator waits and retries later

This prevents the operator from interfering with active backup operations while still recovering from stale locks.

**Configuration via Helm:**

```yaml
# values.yaml
staleLockThreshold: "1h"  # Consider locks stale after 1 hour
```

**Configuration via environment variable:**

```yaml
env:
  - name: STALE_LOCK_THRESHOLD
    value: "1h"
```

**Configuration via command-line flag:**

```bash
/manager --stale-lock-threshold=1h
```

**Recommended values:**

- `30m` (default) - Suitable for most workloads
- `1h` - For larger backups that may take longer
- `2h` - For very large repositories or slow network connections

### Leader Election

For high availability deployments, leader election ensures only one operator instance is active:

```yaml
leaderElection:
  enabled: true
```

## Uninstallation

### Helm

```bash
helm uninstall restic-backup-operator -n backup-system
kubectl delete namespace backup-system
```

### Kustomize

```bash
kubectl delete -k .
kubectl delete namespace backup-system
```
