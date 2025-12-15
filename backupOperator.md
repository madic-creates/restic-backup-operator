# Restic Backup Operator - Requirements Specification

## 1. Overview

### 1.1 Purpose

The Restic Backup Operator is a Kubernetes-native solution for managing restic-based backups of Persistent Volumes and database dumps. It replaces the current shell script-based backup system with a declarative, CRD-driven approach similar to VolSync or Velero, but tailored to the specific needs of this cluster.

### 1.2 Goals

- **Declarative Configuration**: Define backups as Kubernetes Custom Resources
- **Kubernetes-Native**: Full integration with the Kubernetes API and ecosystem
- **Observable**: Status conditions, events, and Prometheus metrics on the CR
- **Flexible**: Support for PVC backups, database dumps, and custom pre/post hooks
- **Secure**: Integration with existing SOPS/age secret management
- **GitOps-Compatible**: Works seamlessly with ArgoCD

### 1.3 Non-Goals

- Replacing Velero for disaster recovery of entire namespaces/clusters
- Backup of etcd or cluster state
- Cross-cluster replication (use dedicated tools for this)

---

## 2. Current State Analysis

### 2.1 Existing Backup Patterns

The current implementation uses three patterns:

#### Pattern A: Sidecar with Crond (emby, nextpvr)

```
Deployment
├── Main Container (application)
└── Sidecar Container (restic + crond)
    ├── ConfigMap: crontab-backup-script (backup.sh)
    ├── ConfigMap: crontab-{app} (cron schedule)
    └── Secret: backup-env-configuration-{app}
```

#### Pattern B: CronJob with InitContainer (mariadb)

```
CronJob
├── InitContainer: kubectl exec mariadb-backup
└── Container: restic backup
    ├── ConfigMap: crontab-backup-script
    └── Secret: backup-env-configuration-{app}
```

#### Pattern C: Global Retention CronJob

```
CronJob (restic-retentionpolicies)
└── Container: restic forget + prune
    └── Secret: restic-backup (global credentials)
```

### 2.2 Current Configuration Parameters

From analyzing `backup.sh` and the encrypted secrets, these parameters are used:

| Parameter | Description | Default |
|-----------|-------------|---------|
| `RESTIC_SOURCE` | Path(s) to backup | Required |
| `RESTIC_REPOSITORY` | S3/Wasabi repository URL | Required |
| `RESTIC_PASSWORD` | Repository encryption password | Required |
| `RESTIC_HOSTNAME` | Hostname for snapshots | Pod hostname |
| `RESTIC_ADDITIONAL_BACKUP_PARAMETERS` | Extra restic flags (e.g., `--tag`) | Empty |
| `RESTIC_BACKUP_COMMAND` | Override entire backup command | Default restic backup |
| `RESTIC_CACHE_DIR` | Cache directory path | Empty |
| `RESTIC_RETENTION_POLICIES_ENABLED` | Run forget after backup | `true` |
| `KEEP_HOURLY` | Retention: hourly snapshots | `24` |
| `KEEP_DAILY` | Retention: daily snapshots | `7` |
| `KEEP_WEEKLY` | Retention: weekly snapshots | `4` |
| `KEEP_MONTHLY` | Retention: monthly snapshots | `12` |
| `KEEP_YEARLY` | Retention: yearly snapshots | `0` |
| `KEEP_LAST` | Retention: minimum snapshots | `1` |
| `PRE_HOOK` | Command to run before backup | Empty |
| `AWS_ACCESS_KEY_ID` | S3 credentials | Required |
| `AWS_SECRET_ACCESS_KEY` | S3 credentials | Required |
| `NTFY_ENABLED` | Enable ntfy notifications | `false` |
| `NTFY_SERVER` | ntfy server URL | `ntfy.sh` |
| `NTFY_TOPIC` | ntfy topic | `backup` |
| `NTFY_CREDS` | ntfy authentication | Empty |
| `NTFY_PRIO` | ntfy priority (1-5) | `4` |
| `NTFY_TAG` | ntfy tags | `bangbang` |
| `NTFY_TITLE` | ntfy notification title | `{hostname} - Backup failed` |
| `PUSHGATEWAY_ENABLED` | Enable Prometheus Pushgateway | `false` |
| `PUSHGATEWAY_URL` | Pushgateway URL | Empty |
| `DEBUG` | Enable shell debug mode | `false` |

---

## 3. Custom Resource Definitions

### 3.1 ResticRepository CRD

Defines a restic repository configuration that can be shared across multiple backups.

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

### 3.2 ResticBackup CRD

Defines a backup job for a specific source.

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
      # Reference to secret with ntfy credentials
      credentialsSecretRef:
        name: ntfy-credentials
        key: auth-header
      # Only notify on failure (default: true)
      onlyOnFailure: true
      priority: 4
      tags:
        - backup
        - warning

    # Email notifications (future)
    # email:
    #   enabled: false
    #   smtpServer: postfix.media.svc:25
    #   recipient: admin@example.com

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

  # === NETWORK POLICY ===
  networkPolicy:
    # Auto-create CiliumNetworkPolicy for backup pods
    enabled: true

    # Additional egress rules beyond defaults
    additionalEgress: []

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

### 3.3 ResticRestore CRD

Defines a restore operation from a backup.

```yaml
apiVersion: backup.resticbackup.io/v1alpha1
kind: ResticRestore
metadata:
  name: emby-restore-20240115
  namespace: media
spec:
  # Reference to the ResticBackup CR (for repository info)
  backupRef:
    name: emby-config-backup

  # Snapshot to restore (default: latest)
  snapshotID: "abc123def456"
  # Or use: snapshotSelector:
  #   latest: true
  #   tags: ["emby"]
  #   hostname: "emby"
  #   before: "2024-01-15T00:00:00Z"

  # === TARGET CONFIGURATION ===
  target:
    # Option A: Restore to existing PVC
    pvc:
      claimName: longhorn-pvc-emby-restored
      # Path within PVC to restore to (default: /)
      path: /

    # Option B: Create new PVC
    # newPVC:
    #   name: emby-restored
    #   storageClassName: longhorn
    #   accessModes:
    #     - ReadWriteOnce
    #   size: 50Gi

  # Paths to restore (default: all)
  includePaths:
    - /config

  # Paths to exclude from restore
  excludePaths:
    - "*.log"

  # Restore options
  options:
    # Overwrite existing files
    overwrite: true
    # Verify restored data
    verify: true

  # Pre/post restore hooks
  hooks:
    preRestore:
      exec:
        podSelector:
          matchLabels:
            app.kubernetes.io/name: emby
        command:
          - /bin/sh
          - -c
          - "supervisorctl stop emby"

    postRestore:
      exec:
        podSelector:
          matchLabels:
            app.kubernetes.io/name: emby
        command:
          - /bin/sh
          - -c
          - "supervisorctl start emby"

status:
  conditions:
    - type: Complete
      status: "True"
      lastTransitionTime: "2024-01-15T10:30:00Z"
      reason: RestoreSucceeded
      message: "Restore completed successfully"

  phase: Completed  # Pending, InProgress, Completed, Failed

  startTime: "2024-01-15T10:25:00Z"
  completionTime: "2024-01-15T10:30:00Z"

  restoredSnapshot: "abc123def456"
  restoredFiles: 12543
  restoredSize: "2.3 GiB"

  # Reference to restore job
  jobRef:
    name: resticrestore-emby-restore-20240115
    namespace: media
```

### 3.4 GlobalRetentionPolicy CRD

Defines cluster-wide or repository-wide retention policies that run independently.

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

---

## 4. Controller Architecture

### 4.1 Controller Components

```
┌─────────────────────────────────────────────────────────────────┐
│                    Restic Backup Operator                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────┐  ┌─────────────────────┐              │
│  │ ResticRepository    │  │ ResticBackup        │              │
│  │ Controller          │  │ Controller          │              │
│  │                     │  │                     │              │
│  │ - Init repository   │  │ - Create CronJobs   │              │
│  │ - Health checks     │  │ - Manage lifecycle  │              │
│  │ - Stats collection  │  │ - Update status     │              │
│  └─────────────────────┘  └─────────────────────┘              │
│                                                                 │
│  ┌─────────────────────┐  ┌─────────────────────┐              │
│  │ ResticRestore       │  │ GlobalRetention     │              │
│  │ Controller          │  │ Controller          │              │
│  │                     │  │                     │              │
│  │ - Create Jobs       │  │ - Create CronJobs   │              │
│  │ - Execute restore   │  │ - Run retention     │              │
│  │ - Verify data       │  │ - Prune repository  │              │
│  └─────────────────────┘  └─────────────────────┘              │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────────┤
│  │ Shared Components                                            │
│  │                                                              │
│  │ - Restic Executor (wrapper for restic CLI)                  │
│  │ - Notification Manager (ntfy, pushgateway, email)           │
│  │ - Metrics Collector (Prometheus metrics)                    │
│  │ - Secret Resolver (fetch credentials from secrets)          │
│  │ - Network Policy Generator (CiliumNetworkPolicy)            │
│  └─────────────────────────────────────────────────────────────┘
└─────────────────────────────────────────────────────────────────┘
```

### 4.2 Reconciliation Logic

#### ResticRepository Controller

```
Reconcile(repository):
  1. Validate spec
  2. Fetch credentials from secretRef
  3. Check if repository exists (restic snapshots)
     - If not: Initialize repository (restic init)
  4. If integrityCheck.enabled and due:
     - Create/update integrity check CronJob
  5. Update status:
     - Set Ready condition
     - Update statistics (restic stats)
  6. Requeue after 1 hour for stats refresh
```

#### ResticBackup Controller

```
Reconcile(backup):
  1. Validate spec
  2. Resolve repositoryRef -> get repository status
     - If repository not Ready: requeue with backoff
  3. Generate CronJob manifest:
     - Build pod spec with restic container
     - Mount source PVC or configure custom source
     - Inject credentials as env vars from secrets
     - Configure hooks as init/sidecar containers
     - Set resource limits, security context
  4. Create/Update CronJob
  5. If networkPolicy.enabled:
     - Generate and apply CiliumNetworkPolicy
  6. Watch for Job completions:
     - On completion: Update status (lastBackup, statistics)
     - On failure: Update status, trigger notifications
  7. Update status conditions
  8. Requeue to update nextBackup time
```

#### ResticRestore Controller

```
Reconcile(restore):
  1. Validate spec
  2. If phase == "":
     - Set phase = Pending
  3. If phase == Pending:
     - Resolve backupRef -> get repository info
     - Resolve snapshotID or find matching snapshot
     - Create restore Job:
       - Run preRestore hook
       - Execute restic restore
       - Run postRestore hook
       - Verify if requested
     - Set phase = InProgress
  4. If phase == InProgress:
     - Watch Job status
     - On completion: Set phase = Completed, update status
     - On failure: Set phase = Failed, update status
  5. Update conditions
```

#### GlobalRetentionPolicy Controller

```
Reconcile(policy):
  1. Validate spec
  2. Resolve repositoryRef
  3. Generate CronJob for retention:
     - For each policy in policies:
       - Build restic forget command with selector
     - If prune: add restic prune
     - Configure notifications
  4. Create/Update CronJob
  5. Watch for Job completions:
     - Update status with results
     - Send notifications
```

### 4.3 Generated Resources

For each `ResticBackup`, the controller generates:

```yaml
# CronJob
apiVersion: batch/v1
kind: CronJob
metadata:
  name: resticbackup-{backup-name}
  namespace: {backup-namespace}
  ownerReferences:
    - apiVersion: backup.resticbackup.io/v1alpha1
      kind: ResticBackup
      name: {backup-name}
      controller: true
spec:
  schedule: "{spec.schedule}"
  concurrencyPolicy: Forbid
  jobTemplate:
    spec:
      template:
        metadata:
          labels:
            backup.resticbackup.io/backup: {backup-name}
            backup.resticbackup.io/type: backup
        spec:
          serviceAccountName: resticbackup-{backup-name}
          initContainers:
            # Pre-backup hook (if defined)
            - name: pre-backup-hook
              ...
          containers:
            - name: restic
              image: ghcr.io/restic/restic:0.18.1
              env:
                - name: RESTIC_REPOSITORY
                  valueFrom:
                    secretKeyRef: ...
                - name: RESTIC_PASSWORD
                  valueFrom:
                    secretKeyRef: ...
              command:
                - /backup-entrypoint.sh
              volumeMounts:
                - name: source-data
                  mountPath: /backup
                  readOnly: true
                - name: backup-script
                  mountPath: /backup-entrypoint.sh
                  subPath: entrypoint.sh
          volumes:
            - name: source-data
              persistentVolumeClaim:
                claimName: {spec.source.pvc.claimName}
            - name: backup-script
              configMap:
                name: restic-operator-scripts
                defaultMode: 0755
          restartPolicy: Never

---
# ServiceAccount (if not specified)
apiVersion: v1
kind: ServiceAccount
metadata:
  name: resticbackup-{backup-name}
  namespace: {backup-namespace}
  ownerReferences: ...

---
# CiliumNetworkPolicy (if enabled)
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: resticbackup-{backup-name}
  namespace: {backup-namespace}
  ownerReferences: ...
spec:
  endpointSelector:
    matchLabels:
      backup.resticbackup.io/backup: {backup-name}
  egress:
    # DNS
    - toEndpoints:
      - matchLabels:
          k8s:io.kubernetes.pod.namespace: kube-system
          k8s:k8s-app: kube-dns
      toPorts:
        - ports:
          - port: "53"
            protocol: UDP
    # S3/Wasabi
    - toEntities:
      - world
      toPorts:
        - ports:
          - port: "443"
            protocol: TCP
    # Pushgateway (if enabled)
    - toEndpoints:
      - matchLabels:
          k8s:io.kubernetes.pod.namespace: monitoring
          k8s:app.kubernetes.io/name: prometheus-pushgateway
      toPorts:
        - ports:
          - port: "9091"
            protocol: TCP
```

---

## 5. Metrics and Observability

### 5.1 Prometheus Metrics

The operator exposes the following metrics:

```
# Backup metrics (pushed to Pushgateway per backup)
backup_duration_seconds{backup="emby-config", namespace="media"} 330
backup_start_timestamp{backup="emby-config", namespace="media"} 1705298400
backup_status{backup="emby-config", namespace="media", status="success"} 1
backup_snapshot_size_bytes{backup="emby-config", namespace="media"} 2469606195
backup_snapshot_files_total{backup="emby-config", namespace="media"} 12543

# Operator metrics (exposed by operator)
restic_operator_reconcile_total{controller="resticbackup", result="success"} 150
restic_operator_reconcile_total{controller="resticbackup", result="error"} 2
restic_operator_reconcile_duration_seconds{controller="resticbackup"} 0.5

# Repository metrics
restic_repository_size_bytes{repository="wasabi-k3s-backup"} 134839066624
restic_repository_snapshots_total{repository="wasabi-k3s-backup"} 156
restic_repository_integrity_check_timestamp{repository="wasabi-k3s-backup"} 1705190400
restic_repository_integrity_check_status{repository="wasabi-k3s-backup"} 1
```

### 5.2 Kubernetes Events

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

### 5.3 Status Conditions

All CRDs use standard Kubernetes conditions:

| Condition | Description |
|-----------|-------------|
| `Ready` | Resource is ready and operating normally |
| `RepositoryReady` | Referenced repository is accessible |
| `Progressing` | Operation is in progress |
| `Degraded` | Resource is operational but experiencing issues |

---

## 6. Security Considerations

### 6.1 RBAC Requirements

```yaml
# Operator ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: restic-operator
rules:
  # CRD management
  - apiGroups: ["backup.resticbackup.io"]
    resources: ["*"]
    verbs: ["*"]
  # CronJob/Job management
  - apiGroups: ["batch"]
    resources: ["cronjobs", "jobs"]
    verbs: ["*"]
  # Secret reading (for credentials)
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list", "watch"]
  # ConfigMap management (for scripts)
  - apiGroups: [""]
    resources: ["configmaps"]
    verbs: ["*"]
  # ServiceAccount management
  - apiGroups: [""]
    resources: ["serviceaccounts"]
    verbs: ["*"]
  # PVC reading (for backup source)
  - apiGroups: [""]
    resources: ["persistentvolumeclaims"]
    verbs: ["get", "list", "watch"]
  # Pod exec (for hooks)
  - apiGroups: [""]
    resources: ["pods", "pods/exec"]
    verbs: ["get", "list", "create"]
  # Events
  - apiGroups: [""]
    resources: ["events"]
    verbs: ["create", "patch"]
  # Network policies
  - apiGroups: ["cilium.io"]
    resources: ["ciliumnetworkpolicies"]
    verbs: ["*"]
```

### 6.2 Secret Management

- Repository credentials are stored in Kubernetes Secrets
- Secrets are encrypted at rest using SOPS/age (GitOps workflow)
- Operator only reads secrets, never writes credentials
- Backup pods receive credentials via environment variables (not mounted files)

### 6.3 Pod Security

Default security context for backup pods:

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65534  # nobody
  fsGroup: 65534
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop:
      - ALL
  seccompProfile:
    type: RuntimeDefault
```

### 6.4 Network Policies

- Auto-generated CiliumNetworkPolicy for each backup
- Default: Allow only DNS, S3 endpoint, and configured services
- Configurable additional egress rules

---

## 7. Installation and Deployment

### 7.1 Deployment Method

The operator is deployed via Kustomize with Helm, following the existing cluster patterns:

```yaml
# apps/restic-operator/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: backup-system

resources:
  - k8s.namespace.yaml
  - k8s.np.restic-operator.yaml

helmCharts:
  - name: restic-operator
    repo: https://charts.example.com  # Or GitHub Pages
    version: 0.1.0
    releaseName: restic-operator
    namespace: backup-system
    valuesFile: values.yaml

generators:
  - kustomize-secret-generator.yaml
```

### 7.2 ArgoCD Integration

```yaml
# apps/argo-cd-apps/15-backup-system-restic-operator.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: restic-operator
  namespace: argocd
  annotations:
    argocd.argoproj.io/sync-wave: "15"
spec:
  project: default
  source:
    repoURL: "example.com"  # Replaced by Kustomize
    targetRevision: main
    path: apps/restic-operator
  destination:
    server: https://kubernetes.default.svc
    namespace: backup-system
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

### 7.3 Migration from Current System

Migration path from existing backup system:

1. Deploy operator alongside existing system
2. Create `ResticRepository` pointing to existing repository
3. Create `ResticBackup` CRs for each workload:
   - Use same tags/hostnames as current system
   - Existing snapshots remain accessible
4. Disable old sidecar containers / CronJobs
5. Remove old ConfigMaps and Secrets

Example migration for emby:

```yaml
# Before: Sidecar in Deployment + ConfigMaps + Secrets
# After:
apiVersion: backup.resticbackup.io/v1alpha1
kind: ResticBackup
metadata:
  name: emby-config
  namespace: media
spec:
  repositoryRef:
    name: wasabi-k3s-backup
    namespace: backup-system
  schedule: "0 2 * * *"
  source:
    pvc:
      claimName: longhorn-pvc-emby
      paths: ["/config"]
  restic:
    hostname: emby
    tags: ["emby"]
  retention:
    enabled: false  # Use GlobalRetentionPolicy instead
  notifications:
    pushgateway:
      enabled: true
      url: http://prometheus-pushgateway.monitoring.svc:9091
    ntfy:
      enabled: true
      serverURL: https://ntfy.example.com
      topic: backups
      credentialsSecretRef:
        name: ntfy-credentials
```

---

## 8. Implementation Plan

### Phase 1: Core CRDs and Controllers

1. Define CRD schemas with OpenAPI validation
2. Implement `ResticRepository` controller
   - Repository initialization
   - Health checks
   - Statistics collection
3. Implement `ResticBackup` controller
   - CronJob generation
   - Status updates
   - Basic notifications (Pushgateway)

### Phase 2: Advanced Features

4. Implement hooks system (preBackup, postBackup, onFailure)
5. Implement `ResticRestore` controller
6. Add ntfy notification support
7. Implement `GlobalRetentionPolicy` controller

### Phase 3: Polish and Migration

8. Auto-generate NetworkPolicies
9. Comprehensive Prometheus metrics
10. Helm chart packaging
11. Documentation and migration guide
12. Testing with existing workloads

---

## 9. Testing Strategy

### 9.1 Unit Tests

- CRD validation
- Reconciliation logic
- Resource generation

### 9.2 Integration Tests

- End-to-end backup/restore with MinIO
- Hook execution
- Notification delivery

### 9.3 E2E Tests in Vagrant

```bash
# Use existing Vagrant environment
vagrant up
export KUBECONFIG="$PWD/shared/k3svm1/k3s.yaml"

# Deploy operator
kubectl apply -k apps/restic-operator

# Create test backup
kubectl apply -f test/e2e/test-backup.yaml

# Trigger manual backup
kubectl create job --from=cronjob/resticbackup-test test-backup-manual

# Verify
kubectl get resticbackup test -o yaml
kubectl logs job/test-backup-manual
```

---

## 10. Future Enhancements

- **Backup Browsing**: Web UI or CLI to browse snapshots and files
- **Cross-Cluster Restore**: Restore to different cluster
- **Backup Verification**: Periodic test restores
- **Deduplication Statistics**: Show dedup efficiency
- **Bandwidth Limiting**: Rate limit backup uploads
- **Parallel Backups**: Run multiple backups concurrently to same repository
- **Backup Windows**: Define allowed backup time windows
- **PVC Snapshots**: Integrate with CSI snapshots for consistent backups

---

## 11. References

- [Restic Documentation](https://restic.readthedocs.io/)
- [Kubernetes Operator Pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
- [Kubebuilder Book](https://book.kubebuilder.io/)
- [VolSync](https://volsync.readthedocs.io/) - Similar project for reference
- [Velero](https://velero.io/) - Backup tool for reference
- Current backup implementation: `apps/backup-script/backup-script.yaml`
