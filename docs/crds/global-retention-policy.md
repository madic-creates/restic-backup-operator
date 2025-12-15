# GlobalRetentionPolicy CRD

Defines cluster-wide or repository-wide retention policies that run independently of individual backups.

## Example

```yaml
apiVersion: backup.resticbackup.io/v1alpha1
kind: GlobalRetentionPolicy
metadata:
  name: weekly-cleanup
  namespace: backup-system
spec:
  # Reference to repository
  repositoryRef:
    name: wasabi-k3s-backup
    namespace: backup-system

  # Schedule for retention run
  schedule: "20 1 * * 0"  # Weekly on Sunday at 1:20 AM

  # Retention policies per tag/hostname
  policies:
    - selector:
        tags: ["emby"]
      retention:
        keepHourly: 48
        keepDaily: 7
        keepWeekly: 2
        keepMonthly: 0
        keepYearly: 0
        keepLast: 4

    - selector:
        tags: ["MariaDB"]
      retention:
        keepHourly: 48
        keepDaily: 7
        keepWeekly: 2
        keepMonthly: 0
        keepYearly: 0
        keepLast: 4

    - selector:
        tags: ["NextPVR"]
      retention:
        keepHourly: 48
        keepDaily: 7
        keepWeekly: 2
        keepMonthly: 0
        keepYearly: 0
        keepLast: 4

  # Run prune after all forget operations
  prune: true

  # Notifications
  notifications:
    email:
      enabled: true
      smtpServer: postfix.media.svc.cluster.local:25
      from: "Restic Operator <no-reply@geekbundle.org>"
      to: "webmaster@geekbundle.org"
      subject: "Restic Retention Policy Report"

status:
  conditions:
    - type: Ready
      status: "True"
      reason: LastRunSucceeded

  lastRun: "2024-01-14T01:20:00Z"
  lastRunResult: "Succeeded"
  lastRunDuration: "15m30s"

  # Statistics
  repositorySizeBefore: "150.2 GiB"
  repositorySizeAfter: "142.8 GiB"
  snapshotsRemoved: 25

  nextRun: "2024-01-21T01:20:00Z"
```

## Spec Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `repositoryRef.name` | string | Yes | Name of ResticRepository |
| `repositoryRef.namespace` | string | No | Namespace of ResticRepository |
| `schedule` | string | Yes | Cron schedule for retention runs |
| `policies` | []PolicyRule | Yes | List of retention policies |
| `prune` | bool | No | Run prune after forget (default: false) |
| `notifications` | NotificationSpec | No | Notification configuration |

### Policy Rules

Each policy rule consists of:

| Field | Type | Description |
|-------|------|-------------|
| `selector.tags` | []string | Match snapshots with these tags |
| `selector.hostname` | string | Match snapshots from this hostname |
| `retention.keepLast` | int | Keep last N snapshots |
| `retention.keepHourly` | int | Keep N hourly snapshots |
| `retention.keepDaily` | int | Keep N daily snapshots |
| `retention.keepWeekly` | int | Keep N weekly snapshots |
| `retention.keepMonthly` | int | Keep N monthly snapshots |
| `retention.keepYearly` | int | Keep N yearly snapshots |

## Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `conditions` | []Condition | Standard Kubernetes conditions |
| `lastRun` | Time | Timestamp of last retention run |
| `lastRunResult` | string | Result (Succeeded/Failed) |
| `lastRunDuration` | Duration | How long the run took |
| `repositorySizeBefore` | string | Repository size before prune |
| `repositorySizeAfter` | string | Repository size after prune |
| `snapshotsRemoved` | int | Number of snapshots removed |
| `nextRun` | Time | Next scheduled run |

## Use Cases

### Centralized Retention Management

Instead of configuring retention on each ResticBackup, use GlobalRetentionPolicy to:
- Apply consistent retention rules across all backups
- Run expensive prune operations during off-peak hours
- Get a single report for all retention activities

### Different Retention per Workload Type

```yaml
policies:
  # Critical databases: longer retention
  - selector:
      tags: ["database", "critical"]
    retention:
      keepDaily: 30
      keepWeekly: 12
      keepMonthly: 12

  # Application configs: shorter retention
  - selector:
      tags: ["config"]
    retention:
      keepDaily: 7
      keepWeekly: 4

  # Media: minimal retention
  - selector:
      tags: ["media"]
    retention:
      keepLast: 3
      keepDaily: 7
```

### Scheduled Prune Operations

Pruning is expensive and can be disruptive. Schedule it separately:

```yaml
spec:
  schedule: "0 4 * * 0"  # 4 AM every Sunday
  prune: true
  policies:
    - selector:
        tags: ["*"]
      retention:
        keepDaily: 7
        keepWeekly: 4
```

## Notifications

### Email Notification

```yaml
notifications:
  email:
    enabled: true
    smtpServer: smtp.example.com:25
    from: "Backup System <backup@example.com>"
    to: "admin@example.com"
    subject: "Weekly Retention Report"
```

The email includes:
- Number of snapshots removed per policy
- Repository size before/after prune
- Any errors encountered
