# ResticRepository CRD

Defines a restic repository configuration that can be shared across multiple backups.

## Example

```yaml
apiVersion: backup.resticbackup.io/v1alpha1
kind: ResticRepository
metadata:
  name: wasabi-k3s-backup
  namespace: backup-system
spec:
  # Repository URL (s3:, sftp:, rest:, etc.)
  repositoryURL: s3:s3.eu-central-1.wasabisys.com/k3s-at-home-01

  # Reference to secret containing credentials
  credentialsSecretRef:
    name: restic-repository-credentials
    # Keys expected in secret:
    # - RESTIC_PASSWORD (required)
    # - AWS_ACCESS_KEY_ID (for S3)
    # - AWS_SECRET_ACCESS_KEY (for S3)

  # Optional: Enable repository integrity checks
  integrityCheck:
    enabled: true
    schedule: "0 3 * * 0"  # Weekly on Sunday at 3 AM

  # Optional: Cache configuration
  cache:
    enabled: true
    # Size limit for cache PVC
    size: 5Gi
    # StorageClass for cache PVC
    storageClassName: longhorn

status:
  # Conditions: Ready, IntegrityCheckSucceeded, IntegrityCheckFailed
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2024-01-15T10:00:00Z"
      reason: RepositoryInitialized
      message: "Repository is initialized and accessible"

  # Last successful integrity check
  lastIntegrityCheck: "2024-01-14T03:00:00Z"
  lastIntegrityCheckResult: "Passed"

  # Repository statistics (from restic stats)
  statistics:
    totalSize: "125.6 GiB"
    totalFileCount: 45632
    snapshotCount: 156
```

## Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `repositoryURL` | string | Yes | Restic repository URL (s3:, sftp:, rest:, etc.) |
| `credentialsSecretRef.name` | string | Yes | Name of the secret containing credentials |
| `integrityCheck.enabled` | bool | No | Enable periodic integrity checks |
| `integrityCheck.schedule` | string | No | Cron schedule for integrity checks |
| `cache.enabled` | bool | No | Enable repository cache |
| `cache.size` | string | No | Size of cache PVC |
| `cache.storageClassName` | string | No | StorageClass for cache PVC |

## Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `conditions` | []Condition | Standard Kubernetes conditions (Ready, IntegrityCheckSucceeded) |
| `lastIntegrityCheck` | Time | Timestamp of last integrity check |
| `lastIntegrityCheckResult` | string | Result of last integrity check (Passed/Failed) |
| `statistics.totalSize` | string | Total repository size |
| `statistics.totalFileCount` | int | Total number of files in repository |
| `statistics.snapshotCount` | int | Total number of snapshots |

## Required Secret Keys

The referenced secret must contain:

| Key | Required | Description |
|-----|----------|-------------|
| `RESTIC_PASSWORD` | Yes | Repository encryption password |
| `AWS_ACCESS_KEY_ID` | For S3 | S3 access key |
| `AWS_SECRET_ACCESS_KEY` | For S3 | S3 secret key |
