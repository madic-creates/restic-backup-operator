# Restic Backup Operator

A Kubernetes operator for managing restic-based backups of Persistent Volumes and database dumps.

## Overview

The Restic Backup Operator is a Kubernetes-native solution for managing restic-based backups. It replaces shell script-based backup systems with a declarative, CRD-driven approach similar to VolSync or Velero, but tailored for simpler backup workflows.

## Features

- **Declarative Configuration**: Define backups as Kubernetes Custom Resources
- **Kubernetes-Native**: Full integration with the Kubernetes API and ecosystem
- **Observable**: Status conditions, events, and Prometheus metrics
- **Flexible**: Support for PVC backups, database dumps, and custom pre/post hooks
- **Secure**: Integration with Kubernetes secrets for credential management
- **GitOps-Compatible**: Works seamlessly with ArgoCD and Flux

## Custom Resources

| CRD | Description |
|-----|-------------|
| `ResticRepository` | Defines a restic repository configuration |
| `ResticBackup` | Defines a backup job with scheduling |
| `ResticRestore` | Defines a restore operation |
| `GlobalRetentionPolicy` | Defines cluster-wide retention policies |

## Installation

### Using Helm

```bash
helm repo add restic-backup-operator https://madic-creates.github.io/restic-backup-operator
helm install restic-backup-operator restic-backup-operator/restic-backup-operator \
  --namespace backup-system \
  --create-namespace
```

### Using Kustomize

```bash
kubectl apply -k config/default
```

## Quick Start

### 1. Create a Repository Secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: restic-credentials
  namespace: backup-system
type: Opaque
stringData:
  RESTIC_PASSWORD: "your-secure-password"
  AWS_ACCESS_KEY_ID: "your-access-key-id"
  AWS_SECRET_ACCESS_KEY: "your-secret-access-key"
```

### 2. Create a ResticRepository

```yaml
apiVersion: backup.resticbackup.io/v1alpha1
kind: ResticRepository
metadata:
  name: my-repository
  namespace: backup-system
spec:
  repositoryURL: s3:s3.eu-central-1.wasabisys.com/my-bucket
  credentialsSecretRef:
    name: restic-credentials
  integrityCheck:
    enabled: true
    schedule: "0 3 * * 0"
```

### 3. Create a ResticBackup

```yaml
apiVersion: backup.resticbackup.io/v1alpha1
kind: ResticBackup
metadata:
  name: my-app-backup
  namespace: default
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
  restic:
    hostname: my-app
    tags:
      - my-app
      - production
```

### 4. Check Status

```bash
kubectl get resticbackups -A
kubectl get resticrepositories -A
```

## Configuration

### ResticBackup Options

| Field | Description | Default |
|-------|-------------|---------|
| `schedule` | Cron schedule for backups | Required |
| `timezone` | Timezone for schedule | UTC |
| `source.pvc.claimName` | PVC to backup | Required |
| `source.pvc.paths` | Paths within PVC | / |
| `source.pvc.excludes` | Patterns to exclude | [] |
| `restic.hostname` | Hostname for snapshots | CR name |
| `restic.tags` | Tags for snapshots | [] |
| `retention.enabled` | Enable retention | false |
| `notifications.pushgateway.enabled` | Enable Pushgateway | false |
| `notifications.ntfy.enabled` | Enable ntfy | false |

### Notifications

The operator supports two notification backends:

- **Prometheus Pushgateway**: Push backup metrics for monitoring
- **ntfy**: Send push notifications on backup completion/failure

## Development

### Prerequisites

- Go 1.25+
- Docker
- kubectl
- kubebuilder

### Build

```bash
make build
```

### Test

```bash
make test
```

### Run Locally

```bash
make install  # Install CRDs
make run      # Run controller locally
```

### Build Docker Image

```bash
make docker-build IMG=ghcr.io/madic-creates/restic-backup-operator:dev
```

## License

Apache License 2.0
