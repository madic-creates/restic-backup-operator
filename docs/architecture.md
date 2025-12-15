# Controller Architecture

## Controller Components

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
│  └─────────────────────────────────────────────────────────────┘
└─────────────────────────────────────────────────────────────────┘
```

## Reconciliation Logic

### ResticRepository Controller

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

### ResticBackup Controller

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
  5. Watch for Job completions:
     - On completion: Update status (lastBackup, statistics)
     - On failure: Update status, trigger notifications
  6. Update status conditions
  7. Requeue to update nextBackup time
```

### ResticRestore Controller

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

### GlobalRetentionPolicy Controller

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

## Generated Resources

For each `ResticBackup`, the controller generates:

### CronJob

```yaml
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
```

### ServiceAccount

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: resticbackup-{backup-name}
  namespace: {backup-namespace}
  ownerReferences: ...
```

