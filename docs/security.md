# Security Considerations

## RBAC Requirements

### Operator ClusterRole

```yaml
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
```

## Secret Management

- Repository credentials are stored in Kubernetes Secrets
- Secrets are encrypted at rest using SOPS/age (GitOps workflow)
- Operator only reads secrets, never writes credentials
- Backup pods receive credentials via environment variables (not mounted files)

### Secret Structure

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: restic-repository-credentials
  namespace: backup-system
type: Opaque
stringData:
  RESTIC_PASSWORD: "your-repository-password"
  AWS_ACCESS_KEY_ID: "your-access-key"
  AWS_SECRET_ACCESS_KEY: "your-secret-key"
```

### Credential Injection

Credentials are injected as environment variables:

```yaml
env:
  - name: RESTIC_PASSWORD
    valueFrom:
      secretKeyRef:
        name: restic-repository-credentials
        key: RESTIC_PASSWORD
  - name: AWS_ACCESS_KEY_ID
    valueFrom:
      secretKeyRef:
        name: restic-repository-credentials
        key: AWS_ACCESS_KEY_ID
```

## Pod Security

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

### Pod Security Standards

The operator supports Pod Security Standards (PSS):
- **Restricted**: Default configuration meets restricted requirements
- **Baseline**: All backup pods meet baseline requirements
- **Privileged**: Not required for any functionality

### Custom Security Context

Override defaults in ResticBackup:

```yaml
spec:
  jobConfig:
    securityContext:
      runAsNonRoot: true
      runAsUser: 1000
      fsGroup: 1000
```

## Network Policies

Network policy management is left to the Kubernetes operator/administrator. Backup pods use consistent labels to enable matching with generic network policies (e.g., Cilium):

- `backup.resticbackup.io/backup`: Name of the ResticBackup resource
- `backup.resticbackup.io/type`: Type of operation (backup, restore)

This approach allows:
- Integration with existing cluster-wide network policies
- Use of CNI-specific features (Cilium, Calico, etc.)
- Centralized security management by cluster administrators

## Best Practices

### Least Privilege

1. Use dedicated ServiceAccounts per backup
2. Limit secret access to required namespaces
3. Apply appropriate network policies using pod labels

### Secret Rotation

1. Rotate S3 credentials periodically
2. Update secrets through GitOps (SOPS/age)
3. No manual secret changes in cluster

### Audit Trail

1. Enable Kubernetes audit logging
2. Monitor operator events
3. Track backup success/failure metrics

### Backup Security

1. Use strong RESTIC_PASSWORD
2. Enable repository integrity checks
3. Store repository in separate cloud account
