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

# Default restic image for backup jobs
restic:
  image: restic/restic:0.17.3

# Operator configuration
config:
  # Watch all namespaces (empty list = all)
  watchNamespaces: []
  # Leader election for HA
  leaderElection: true
  # Metrics port
  metricsPort: 8080
  # Health probe port
  healthPort: 8081
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
