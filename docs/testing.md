# Testing Strategy

## Unit Tests

- CRD validation
- Reconciliation logic
- Resource generation

### Running Unit Tests

```bash
make test
```

### Running Single Test File

```bash
KUBEBUILDER_ASSETS="$(bin/setup-envtest use 1.31.0 --bin-dir bin -p path)" \
  go test ./internal/controller/ -run TestControllerName -v
```

## Integration Tests

- End-to-end backup/restore with MinIO
- Hook execution
- Notification delivery

### Test Environment

Integration tests use envtest with an embedded Kubernetes API server.

```go
// internal/controller/suite_test.go
var _ = BeforeSuite(func() {
    testEnv = &envtest.Environment{
        CRDDirectoryPaths: []string{
            filepath.Join("..", "..", "config", "crd", "bases"),
        },
    }
    cfg, err = testEnv.Start()
    // ...
})
```

## E2E Tests in Vagrant

Use the existing Vagrant environment for full end-to-end testing:

```bash
# Start Vagrant environment
vagrant up

# Configure kubectl
export KUBECONFIG="$PWD/shared/k3svm1/k3s.yaml"

# Deploy operator
kubectl apply -k apps/restic-operator

# Create test backup
kubectl apply -f test/e2e/test-backup.yaml

# Trigger manual backup
kubectl create job --from=cronjob/resticbackup-test test-backup-manual

# Verify results
kubectl get resticbackup test -o yaml
kubectl logs job/test-backup-manual
```

## Test Categories

### Controller Tests

| Test | Description |
|------|-------------|
| ResticRepository creation | Verify repository initialization |
| ResticBackup CronJob | Verify CronJob is created correctly |
| Status updates | Verify status conditions are set |
| Finalizer handling | Verify cleanup on deletion |

### Restic Executor Tests

| Test | Description |
|------|-------------|
| Init command | Verify restic init parameters |
| Backup command | Verify backup with tags, excludes |
| Restore command | Verify restore paths, options |
| Forget command | Verify retention parameters |

### Notification Tests

| Test | Description |
|------|-------------|
| Pushgateway push | Verify metrics are pushed |
| ntfy notification | Verify notification content |
| Error handling | Verify failures are reported |

## Linting

```bash
# Run linter
make lint

# Auto-fix issues
make lint-fix
```

## Coverage

Generate coverage report:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## CI Pipeline

Tests are run automatically on:
- Pull requests
- Pushes to main branch

See [CI Configuration](ci-configuration.md) for details.
