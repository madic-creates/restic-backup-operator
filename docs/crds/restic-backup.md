# ResticBackup CRD

Defines a backup job for a specific source. The operator creates and manages a CronJob based on this specification.

## Example

```yaml
apiVersion: backup.resticbackup.io/v1alpha1
kind: ResticBackup
metadata:
  name: emby-config-backup
  namespace: media
spec:
  # Reference to ResticRepository (can be in different namespace)
  repositoryRef:
    name: wasabi-k3s-backup
    namespace: backup-system

  # Backup schedule (cron format)
  schedule: "0 2 * * *"

  # Timezone for schedule interpretation
  timezone: "Europe/Berlin"

  # === SOURCE CONFIGURATION ===
  source:
    # Option A: Backup from existing PVC
    pvc:
      claimName: longhorn-pvc-emby
      # Paths within the PVC to backup (default: /)
      paths:
        - /config
        - /data
      # Paths to exclude
      excludes:
        - "*.log"
        - "cache/**"
        - "transcoding-temp/**"

    # Option B: Backup from a pod's volume
    # podVolumeBackup:
    #   selector:
    #     matchLabels:
    #       app.kubernetes.io/name: emby
    #   volumeName: config
    #   container: emby  # Optional: specific container

    # Option C: Custom source (for database dumps etc.)
    # customSource:
    #   # Pod spec for backup job
    #   podTemplate:
    #     spec:
    #       containers:
    #         - name: backup
    #           image: mariadb:10.11
    #           command: ["/bin/sh", "-c"]
    #           args:
    #             - |
    #               mariadb-dump ... > /backup/dump.sql
    #   # Path in pod where backup data will be written
    #   backupPath: /backup

  # === RESTIC CONFIGURATION ===
  restic:
    # Hostname for snapshots (default: CR name)
    hostname: emby

    # Tags for this backup
    tags:
      - emby
      - media
      - config

    # Additional restic backup arguments
    extraArgs:
      - "--exclude-caches"
      - "--one-file-system"

    # Container image for restic
    image: ghcr.io/restic/restic:0.18.1

  # === HOOKS ===
  hooks:
    # Pre-backup hook (runs before restic backup)
    preBackup:
      # Option A: Execute command in existing pod
      exec:
        podSelector:
          matchLabels:
            app.kubernetes.io/name: emby
        container: emby
        command:
          - /bin/sh
          - -c
          - "echo 'Preparing backup...'"
        timeout: 60s

      # Option B: Run a job before backup
      # job:
      #   podTemplate:
      #     spec:
      #       containers:
      #         - name: pre-hook
      #           image: busybox
      #           command: ["echo", "pre-backup"]

    # Post-backup hook (runs after successful backup)
    postBackup:
      exec:
        podSelector:
          matchLabels:
            app.kubernetes.io/name: emby
        container: emby
        command:
          - /bin/sh
          - -c
          - "echo 'Backup completed'"
        timeout: 30s

    # On-failure hook
    onFailure:
      exec:
        podSelector:
          matchLabels:
            app.kubernetes.io/name: emby
        container: emby
        command:
          - /bin/sh
          - -c
          - "echo 'Backup failed!' >&2"

  # === RETENTION POLICY ===
  retention:
    # Apply retention after each backup
    enabled: true

    # Retention rules (restic forget parameters)
    policy:
      keepLast: 4
      keepHourly: 48
      keepDaily: 7
      keepWeekly: 4
      keepMonthly: 6
      keepYearly: 1

    # Prune after forget (can be expensive)
    prune: false

    # Group by for retention (default: host,tags)
    groupBy:
      - host
      - tags

  # === NOTIFICATIONS ===
  notifications:
    # Prometheus Pushgateway
    pushgateway:
      enabled: true
      url: http://prometheus-pushgateway.monitoring.svc:9091
      # Job name in Pushgateway (default: backup)
      jobName: backup

    # ntfy push notifications
    ntfy:
      enabled: true
      serverURL: https://ntfy.example.com
      topic: backups
      # Reference to secret with ntfy credentials (see Ntfy Credentials section below)
      credentialsSecretRef:
        name: ntfy-credentials
        namespace: backup-system  # optional, defaults to same namespace
      # Only notify on failure (default: false)
      onlyOnFailure: true
      priority: 4
      tags:
        - backup
        - warning

  # === JOB CONFIGURATION ===
  jobConfig:
    # Concurrency policy for CronJob
    concurrencyPolicy: Forbid

    # Number of successful/failed jobs to keep
    successfulJobsHistoryLimit: 3
    failedJobsHistoryLimit: 3

    # Job timeout
    activeDeadlineSeconds: 3600

    # Backoff limit for failed jobs
    backoffLimit: 0

    # Pod security context
    securityContext:
      runAsNonRoot: true
      runAsUser: 1000
      fsGroup: 1000
      seccompProfile:
        type: RuntimeDefault

    # Resource requests/limits for backup pod
    resources:
      requests:
        memory: "128Mi"
        cpu: "100m"
      limits:
        memory: "512Mi"
        cpu: "1000m"

    # Node selector
    nodeSelector:
      kubernetes.io/arch: amd64

    # Tolerations
    tolerations: []

    # Affinity rules
    affinity: {}

    # Service account (auto-created if not specified)
    serviceAccountName: ""

  # Suspend scheduling (useful for maintenance)
  suspend: false

status:
  # Overall conditions
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

  # Last backup information
  lastBackup:
    startTime: "2024-01-15T02:00:00Z"
    completionTime: "2024-01-15T02:05:30Z"
    duration: "5m30s"
    snapshotID: "abc123def456"
    result: "Succeeded"  # Succeeded, Failed, PartiallyFailed

  # Last successful backup
  lastSuccessfulBackup: "2024-01-15T02:00:00Z"

  # Next scheduled backup
  nextBackup: "2024-01-16T02:00:00Z"

  # Backup statistics
  statistics:
    totalBackups: 45
    successfulBackups: 44
    failedBackups: 1
    lastBackupSize: "2.3 GiB"
    lastBackupFiles: 12543

  # Retention status
  lastRetentionRun: "2024-01-15T02:05:00Z"
  snapshotsAfterRetention: 15

  # Reference to managed CronJob
  cronJobRef:
    name: resticbackup-emby-config-backup
    namespace: media

  # Observed generation for reconciliation tracking
  observedGeneration: 3
```

## Source Types

### PVC Source

Backup from an existing PersistentVolumeClaim:

```yaml
source:
  pvc:
    claimName: my-pvc
    paths:
      - /data
    excludes:
      - "*.tmp"
```

### Pod Volume Source

Backup from a volume mounted in a running pod:

```yaml
source:
  podVolumeBackup:
    selector:
      matchLabels:
        app: myapp
    volumeName: data
    container: main
```

### Custom Source

Run a custom container to generate backup data (e.g., database dumps):

```yaml
source:
  customSource:
    podTemplate:
      spec:
        containers:
          - name: dump
            image: mariadb:10.11
            command: ["/bin/sh", "-c"]
            args:
              - mariadb-dump --all-databases > /backup/dump.sql
    backupPath: /backup
```

## Hooks

Hooks allow running commands before/after backups:

| Hook | When | Use Case |
|------|------|----------|
| `preBackup` | Before restic backup | Stop application, flush caches |
| `postBackup` | After successful backup | Restart application, cleanup |
| `onFailure` | On backup failure | Alert, rollback |

Each hook supports:
- `exec`: Execute command in existing pod
- `job`: Run a separate job

## Retention Policy

Configure snapshot retention using restic forget parameters:

| Field | Description |
|-------|-------------|
| `keepLast` | Keep last N snapshots |
| `keepHourly` | Keep N hourly snapshots |
| `keepDaily` | Keep N daily snapshots |
| `keepWeekly` | Keep N weekly snapshots |
| `keepMonthly` | Keep N monthly snapshots |
| `keepYearly` | Keep N yearly snapshots |
| `prune` | Run prune after forget |
| `groupBy` | Group snapshots by host/tags |

## Ntfy Credentials

The `spec.notifications.ntfy.credentialsSecretRef` references a Kubernetes Secret containing ntfy authentication credentials. The secret can be in the same namespace as the ResticBackup or in a different namespace (specify `namespace` field).

### Secret Format

The secret can contain the following keys:

| Key | Description | Priority |
|-----|-------------|----------|
| `token` | Bearer token for authentication | Preferred (used if present) |
| `username` | Username for basic authentication | Used with `password` if no `token` |
| `password` | Password for basic authentication | Used with `username` if no `token` |

If `token` is present, it will be used as a Bearer token. Otherwise, `username` and `password` will be used for Basic authentication.

### Example: Bearer Token Authentication

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ntfy-credentials
  namespace: backup-system
type: Opaque
stringData:
  token: "tk_your_ntfy_access_token"
```

### Example: Basic Authentication

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ntfy-credentials
  namespace: backup-system
type: Opaque
stringData:
  username: "your-username"
  password: "your-password"
```

### Usage in ResticBackup

```yaml
spec:
  notifications:
    ntfy:
      enabled: true
      serverURL: https://ntfy.example.com
      topic: backups
      credentialsSecretRef:
        name: ntfy-credentials
        namespace: backup-system  # optional
```
