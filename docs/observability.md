# Metrics and Observability

## Prometheus Metrics

The operator exposes the following metrics:

### Backup Metrics (pushed to Pushgateway per backup)

```
backup_duration_seconds{backup="emby-config", namespace="media"} 330
backup_start_timestamp{backup="emby-config", namespace="media"} 1705298400
backup_status{backup="emby-config", namespace="media", status="success"} 1
backup_snapshot_size_bytes{backup="emby-config", namespace="media"} 2469606195
backup_snapshot_files_total{backup="emby-config", namespace="media"} 12543
```

### Operator Metrics (exposed by operator)

```
restic_operator_reconcile_total{controller="resticbackup", result="success"} 150
restic_operator_reconcile_total{controller="resticbackup", result="error"} 2
restic_operator_reconcile_duration_seconds{controller="resticbackup"} 0.5
```

### Repository Metrics

```
restic_repository_size_bytes{repository="wasabi-k3s-backup"} 134839066624
restic_repository_snapshots_total{repository="wasabi-k3s-backup"} 156
restic_repository_integrity_check_timestamp{repository="wasabi-k3s-backup"} 1705190400
restic_repository_integrity_check_status{repository="wasabi-k3s-backup"} 1
```

## Kubernetes Events

The operator emits events for important state changes:

```
Events:
  Type     Reason              Message
  ----     ------              -------
  Normal   BackupStarted       Starting backup for snapshot
  Normal   BackupCompleted     Backup completed successfully (snapshot: abc123)
  Warning  BackupFailed        Backup failed: connection timeout to S3
  Normal   RetentionCompleted  Retention policy applied, removed 5 snapshots
  Warning  RepositoryUnhealthy Repository integrity check failed
  Normal   RestoreCompleted    Restore completed successfully
```

## Status Conditions

All CRDs use standard Kubernetes conditions:

| Condition | Description |
|-----------|-------------|
| `Ready` | Resource is ready and operating normally |
| `RepositoryReady` | Referenced repository is accessible |
| `Progressing` | Operation is in progress |
| `Degraded` | Resource is operational but experiencing issues |

### Example Status

```yaml
status:
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2024-01-15T02:00:00Z"
      reason: BackupSucceeded
      message: "Last backup completed successfully"
    - type: RepositoryReady
      status: "True"
      lastTransitionTime: "2024-01-15T01:00:00Z"
      reason: RepositoryAccessible
      message: "Repository is accessible"
```

## Pushgateway Integration

Configure Pushgateway in ResticBackup:

```yaml
spec:
  notifications:
    pushgateway:
      enabled: true
      url: http://prometheus-pushgateway.monitoring.svc:9091
      jobName: backup
```

Metrics pushed after each backup:
- `backup_last_success_timestamp`
- `backup_duration_seconds`
- `backup_size_bytes`
- `backup_files_total`

## ntfy Notifications

Configure ntfy push notifications:

```yaml
spec:
  notifications:
    ntfy:
      enabled: true
      serverURL: https://ntfy.example.com
      topic: backups
      credentialsSecretRef:
        name: ntfy-credentials
        key: auth-header
      onlyOnFailure: true
      priority: 4
      tags:
        - backup
        - warning
```

Notification content includes:
- Backup name and namespace
- Status (success/failure)
- Duration
- Error message (on failure)
- Snapshot ID (on success)

## Alerting Recommendations

### Prometheus AlertManager Rules

```yaml
groups:
  - name: restic-backup
    rules:
      - alert: BackupFailed
        expr: backup_status{status="failed"} == 1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Backup {{ $labels.backup }} failed"

      - alert: BackupMissing
        expr: time() - backup_last_success_timestamp > 86400 * 2
        for: 1h
        labels:
          severity: warning
        annotations:
          summary: "No successful backup for {{ $labels.backup }} in 2 days"

      - alert: RepositoryUnhealthy
        expr: restic_repository_integrity_check_status == 0
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Repository {{ $labels.repository }} integrity check failed"
```
