# Restic Backup Operator Documentation

This documentation describes the Restic Backup Operator, a Kubernetes-native solution for managing restic-based backups of Persistent Volumes and database dumps.

## Table of Contents

### Getting Started
- [Overview](overview.md) - Purpose, goals, and scope of the operator
- [Installation](installation.md) - Deployment and ArgoCD integration

### Custom Resource Definitions
- [ResticRepository](crds/restic-repository.md) - Repository configuration
- [ResticBackup](crds/restic-backup.md) - Scheduled backup jobs
- [ResticRestore](crds/restic-restore.md) - Restore operations
- [GlobalRetentionPolicy](crds/global-retention-policy.md) - Cluster-wide retention rules

### Architecture & Operations
- [Controller Architecture](architecture.md) - Controller components and reconciliation logic
- [Security](security.md) - RBAC, secrets, and pod security
- [Observability](observability.md) - Metrics, events, and status conditions

### Development
- [Current State Analysis](current-state.md) - Analysis of the existing backup system
- [Implementation Plan](implementation-plan.md) - Development phases
- [Testing Strategy](testing.md) - Unit, integration, and E2E testing
- [Roadmap](roadmap.md) - Future enhancements

### Additional Resources
- [CI Configuration](ci-configuration.md) - CI/CD pipeline setup
- [Comparison](comparison.md) - Comparison with other backup solutions

## References

- [Restic Documentation](https://restic.readthedocs.io/)
- [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [Kubebuilder Book](https://book.kubebuilder.io/)
- [VolSync](https://volsync.readthedocs.io/) - Similar project for reference
- [Velero](https://velero.io/) - Backup tool for reference
