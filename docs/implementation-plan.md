# Implementation Plan

## Phase 1: Core CRDs and Controllers

### 1. Define CRD schemas with OpenAPI validation

- ResticRepository CRD
- ResticBackup CRD
- ResticRestore CRD
- GlobalRetentionPolicy CRD

### 2. Implement ResticRepository Controller

- Repository initialization
- Health checks
- Statistics collection

### 3. Implement ResticBackup Controller

- CronJob generation
- Status updates
- Basic notifications (Pushgateway)

## Phase 2: Advanced Features

### 4. Implement Hooks System

- preBackup hooks
- postBackup hooks
- onFailure hooks

### 5. Implement ResticRestore Controller

- Snapshot selection
- Restore job creation
- Verification

### 6. Add ntfy Notification Support

- ntfy client integration
- Credential handling
- Notification templates

### 7. Implement GlobalRetentionPolicy Controller

- Multi-policy support
- Scheduled pruning
- Statistics tracking

## Phase 3: Polish and Migration

### 8. Comprehensive Prometheus Metrics

- Operator metrics
- Repository metrics
- Backup job metrics

### 9. Helm Chart Packaging

- Chart structure
- Values configuration
- Documentation

### 10. Documentation and Migration Guide

- User documentation
- Migration procedures
- Troubleshooting guide

### 11. Testing with Existing Workloads

- Integration testing
- Performance testing
- Rollback procedures
