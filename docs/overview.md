# Overview

## Purpose

The Restic Backup Operator is a Kubernetes-native solution for managing restic-based backups of Persistent Volumes and database dumps. It specifies a backup system with a declarative, CRD-driven approach similar to VolSync or Velero.

## Goals

- **Declarative Configuration**: Define backups as Kubernetes Custom Resources
- **Kubernetes-Native**: Full integration with the Kubernetes API and ecosystem
- **Observable**: Status conditions, events, and Prometheus metrics on the CR
- **Flexible**: Support for PVC backups, database dumps, and custom pre/post hooks
- **Secure**: Integration with existing SOPS/age secret management
- **GitOps-Compatible**: Works seamlessly with ArgoCD

## Non-Goals

- Replacing Velero for disaster recovery of entire namespaces/clusters
- Backup of etcd or cluster state
- Cross-cluster replication (use dedicated tools for this)
