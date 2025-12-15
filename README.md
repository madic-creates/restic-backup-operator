# Restic Backup Operator

A Kubernetes operator for managing restic-based backups of Persistent Volumes and database dumps.

## Overview

The Restic Backup Operator is a Kubernetes-native solution for managing restic-based backups. It replaces shell script-based backup systems with a declarative, CRD-driven approach similar to VolSync or Velero, but tailored for simpler backup workflows.

### Key Features

- **Declarative Configuration**: Define backups as Kubernetes Custom Resources
- **Kubernetes-Native**: Full integration with the Kubernetes API and ecosystem
- **Observable**: Status conditions, events, and Prometheus metrics
- **Flexible**: Support for PVC backups, database dumps, and custom pre/post hooks
- **Secure**: Integration with Kubernetes secrets for credential management
- **GitOps-Compatible**: Works seamlessly with ArgoCD and Flux

## Documentation

For detailed documentation, see the [docs/](docs/README.md) directory:

- [Overview](docs/overview.md) - Purpose, goals, and scope
- [Installation](docs/installation.md) - Deployment methods and configuration
- [Architecture](docs/architecture.md) - Controller components and design

### Custom Resource Definitions

- [ResticRepository](docs/crds/restic-repository.md) - Repository configuration
- [ResticBackup](docs/crds/restic-backup.md) - Scheduled backup jobs
- [ResticRestore](docs/crds/restic-restore.md) - Restore operations
- [GlobalRetentionPolicy](docs/crds/global-retention-policy.md) - Cluster-wide retention rules

## Quick Start

```bash
# Install with Helm
helm repo add restic-backup-operator https://madic-creates.github.io/restic-backup-operator
helm install restic-backup-operator restic-backup-operator/restic-backup-operator \
  --namespace backup-system \
  --create-namespace

# Verify installation
kubectl get crds | grep resticbackup
```

For complete installation instructions and examples, see the [Installation Guide](docs/installation.md).

## Development

```bash
make build      # Build manager binary
make test       # Run unit tests
make lint       # Run linter
make install    # Install CRDs to cluster
make run        # Run controller locally
```

For more details, see [CLAUDE.md](CLAUDE.md) or the [Testing Strategy](docs/testing.md).

## License

Apache License 2.0
