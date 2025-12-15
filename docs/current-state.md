# Current State Analysis

This document analyzes the existing backup implementation to understand patterns and configuration parameters that the operator needs to support.

## Existing Backup Patterns

The current implementation uses three patterns:

### Pattern A: Sidecar with Crond (emby, nextpvr)

```
Deployment
├── Main Container (application)
└── Sidecar Container (restic + crond)
    ├── ConfigMap: crontab-backup-script (backup.sh)
    ├── ConfigMap: crontab-{app} (cron schedule)
    └── Secret: backup-env-configuration-{app}
```

### Pattern B: CronJob with InitContainer (mariadb)

```
CronJob
├── InitContainer: kubectl exec mariadb-backup
└── Container: restic backup
    ├── ConfigMap: crontab-backup-script
    └── Secret: backup-env-configuration-{app}
```

### Pattern C: Global Retention CronJob

```
CronJob (restic-retentionpolicies)
└── Container: restic forget + prune
    └── Secret: restic-backup (global credentials)
```

## Current Configuration Parameters

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
