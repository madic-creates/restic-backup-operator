# ResticRestore CRD

Defines a restore operation from a backup.

## Example

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

## Spec Fields

### Snapshot Selection

| Field | Type | Description |
|-------|------|-------------|
| `backupRef.name` | string | Name of ResticBackup CR for repository info |
| `snapshotID` | string | Specific snapshot ID to restore |
| `snapshotSelector.latest` | bool | Select the latest snapshot |
| `snapshotSelector.tags` | []string | Filter by tags |
| `snapshotSelector.hostname` | string | Filter by hostname |
| `snapshotSelector.before` | Time | Select snapshot before this time |

### Target Configuration

| Field | Type | Description |
|-------|------|-------------|
| `target.pvc.claimName` | string | Existing PVC to restore to |
| `target.pvc.path` | string | Path within PVC (default: /) |
| `target.newPVC.name` | string | Name for new PVC |
| `target.newPVC.storageClassName` | string | StorageClass for new PVC |
| `target.newPVC.accessModes` | []string | Access modes for new PVC |
| `target.newPVC.size` | string | Size of new PVC |

### Restore Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `includePaths` | []string | all | Paths to restore |
| `excludePaths` | []string | none | Paths to exclude |
| `options.overwrite` | bool | false | Overwrite existing files |
| `options.verify` | bool | false | Verify restored data |

## Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Pending, InProgress, Completed, Failed |
| `conditions` | []Condition | Standard Kubernetes conditions |
| `startTime` | Time | When restore started |
| `completionTime` | Time | When restore completed |
| `restoredSnapshot` | string | Snapshot ID that was restored |
| `restoredFiles` | int | Number of files restored |
| `restoredSize` | string | Size of restored data |
| `jobRef` | ObjectReference | Reference to restore job |

## Workflow

1. Create ResticRestore CR
2. Operator sets phase to `Pending`
3. Operator resolves snapshot (by ID or selector)
4. Operator runs preRestore hook (if defined)
5. Operator creates restore Job, sets phase to `InProgress`
6. Job completes restore
7. Operator runs postRestore hook (if defined)
8. Operator sets phase to `Completed` or `Failed`

## Common Use Cases

### Restore Latest Snapshot

```yaml
spec:
  backupRef:
    name: my-backup
  snapshotSelector:
    latest: true
  target:
    pvc:
      claimName: my-pvc
```

### Restore to New PVC

```yaml
spec:
  backupRef:
    name: my-backup
  snapshotID: "abc123"
  target:
    newPVC:
      name: restored-data
      storageClassName: longhorn
      accessModes:
        - ReadWriteOnce
      size: 10Gi
```

### Partial Restore

```yaml
spec:
  backupRef:
    name: my-backup
  snapshotSelector:
    latest: true
  includePaths:
    - /config
  excludePaths:
    - /config/cache
  target:
    pvc:
      claimName: my-pvc
```
