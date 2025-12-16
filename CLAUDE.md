# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Kubernetes operator for managing restic-based backups. Built with Kubebuilder/controller-runtime framework. Replaces shell script-based backup systems with declarative CRD-driven approach.

## Common Commands

```bash
# Build and validate
make build              # Build manager binary (includes manifests, generate, fmt, vet)
make manifests          # Generate CRD and RBAC manifests
make generate           # Generate DeepCopy methods

# Testing
make test               # Run unit tests with envtest (includes manifests, generate, fmt, vet)
make test-e2e           # Run end-to-end tests

# Run single test file
KUBEBUILDER_ASSETS="$(bin/setup-envtest use 1.31.0 --bin-dir bin -p path)" go test ./internal/controller/ -run TestControllerName -v

# Linting
make lint               # Run golangci-lint
make lint-fix           # Auto-fix lint issues

# Local development
make install            # Install CRDs to cluster
make run                # Run controller locally against cluster
make uninstall          # Remove CRDs from cluster

# Docker
make docker-build IMG=<registry>/<image>:<tag>
make docker-push IMG=<registry>/<image>:<tag>

# Helm
make helm-crds          # Copy CRDs to Helm chart
make helm-lint          # Validate Helm chart
make helm-install       # Install chart to backup-system namespace
```

## Architecture

### Custom Resource Definitions (api/v1alpha1/)
- **ResticRepository**: Repository configuration (URL, credentials, integrity checks, cache)
- **ResticBackup**: Scheduled backup jobs (creates CronJobs, handles retention, notifications)
- **ResticRestore**: Restore operations (snapshot selection, target PVC handling)
- **GlobalRetentionPolicy**: Cluster-wide retention rules

### Controllers (internal/controller/)
Each CRD has a reconciler implementing the standard Kubernetes controller pattern:
- Watch for resource changes
- Drive towards desired state
- Update status conditions
- Record events for audit trail

Key patterns:
- Finalizers used for cleanup on deletion (e.g., `resticbackup-finalizer`)
- Conditions express resource health: Ready, Progressing, Degraded, RepositoryReady
- ResticBackup creates/manages CronJobs for scheduled execution
- Cross-namespace references supported (ResticBackup can reference Repository in different namespace)

### Restic Integration (internal/restic/)
- **Executor interface**: Init, Check, Stats, Snapshots, Backup, Restore, Forget, Prune
- **DefaultExecutor**: Wraps restic CLI, parses JSON output, handles credentials via environment variables

### Notifications (internal/notifications/)
- **Manager**: Orchestrates notifications to multiple backends
- **Pushgateway**: Prometheus metrics push
- **Ntfy**: ntfy.sh push notifications

### Status Management (internal/conditions/)
Utilities for Kubernetes condition management with proper LastTransitionTime handling.

## Backup Source Types
- **PVC**: Mount existing PersistentVolumeClaim
- **Pod Volume**: Access volume from running pod
- **Custom**: User-defined container with custom commands

## Testing
Uses Ginkgo v2 + Gomega with envtest for embedded Kubernetes API server. Test setup in `internal/controller/suite_test.go` bootstraps all controllers.

## Tool Versions
- Go: 1.25
- Kubernetes (envtest): 1.31.0
- controller-tools: v0.18.0
- golangci-lint: v2.7.2

## Language Requirements
- All commits, comments, documentation, and code must be written in English

## CLAUDE.md Maintenance
This file should be automatically updated with important changes:
- New CRDs or controllers
- Changed architecture decisions
- New commands or tool versions
- Important conventions or patterns
