/*
Copyright 2024 madic-creates.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package notifications

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
)

// EventType represents the type of notification event.
type EventType string

const (
	// EventTypeSuccess indicates a successful operation.
	EventTypeSuccess EventType = "success"
	// EventTypeFailure indicates a failed operation.
	EventTypeFailure EventType = "failure"
	// EventTypeWarning indicates a warning.
	EventTypeWarning EventType = "warning"
)

// Event represents a notification event.
type Event struct {
	Type       EventType
	Resource   string
	Namespace  string
	Message    string
	Details    map[string]string
	Timestamp  time.Time
	Duration   time.Duration
	SnapshotID string
	Size       string
	Files      int64
}

// Config contains configuration for all notification backends.
type Config struct {
	// Pushgateway configuration
	Pushgateway *PushgatewayConfig
	// Ntfy configuration
	Ntfy *NtfyConfig
}

// PushgatewayConfig contains Pushgateway configuration.
type PushgatewayConfig struct {
	URL     string
	JobName string
}

// NtfyConfig contains ntfy configuration.
type NtfyConfig struct {
	ServerURL     string
	Topic         string
	AuthHeader    string
	OnlyOnFailure bool
	Priority      int32
	Tags          []string
}

// Manager coordinates sending notifications to multiple backends.
type Manager struct {
	log         logr.Logger
	ntfy        *NtfyNotifier
	pushgateway *PushgatewayNotifier
}

// NewManager creates a new notification manager.
func NewManager(log logr.Logger) *Manager {
	return &Manager{
		log:         log,
		ntfy:        NewNtfyNotifier(log),
		pushgateway: NewPushgatewayNotifier(log),
	}
}

// Notify sends a notification to all configured backends.
func (m *Manager) Notify(ctx context.Context, config Config, event Event) error {
	var errs []error

	// Send to Pushgateway
	if config.Pushgateway != nil && config.Pushgateway.URL != "" {
		if err := m.pushgateway.Notify(ctx, *config.Pushgateway, event); err != nil {
			m.log.Error(err, "Failed to send notification to Pushgateway")
			errs = append(errs, fmt.Errorf("pushgateway: %w", err))
		}
	}

	// Send to ntfy
	if config.Ntfy != nil && config.Ntfy.ServerURL != "" {
		// Check if we should send based on onlyOnFailure setting
		if !config.Ntfy.OnlyOnFailure || event.Type == EventTypeFailure {
			if err := m.ntfy.Notify(ctx, *config.Ntfy, event); err != nil {
				m.log.Error(err, "Failed to send notification to ntfy")
				errs = append(errs, fmt.Errorf("ntfy: %w", err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("notification errors: %v", errs)
	}

	return nil
}

// NotifyBackupSuccess sends a backup success notification.
func (m *Manager) NotifyBackupSuccess(ctx context.Context, config Config, resource, namespace, snapshotID, size string, files int64, duration time.Duration) error {
	event := Event{
		Type:       EventTypeSuccess,
		Resource:   resource,
		Namespace:  namespace,
		Message:    fmt.Sprintf("Backup completed successfully: %s", snapshotID),
		Timestamp:  time.Now(),
		Duration:   duration,
		SnapshotID: snapshotID,
		Size:       size,
		Files:      files,
		Details: map[string]string{
			"snapshot_id": snapshotID,
			"size":        size,
			"files":       fmt.Sprintf("%d", files),
		},
	}
	return m.Notify(ctx, config, event)
}

// NotifyBackupFailure sends a backup failure notification.
func (m *Manager) NotifyBackupFailure(ctx context.Context, config Config, resource, namespace, errorMsg string, duration time.Duration) error {
	event := Event{
		Type:      EventTypeFailure,
		Resource:  resource,
		Namespace: namespace,
		Message:   fmt.Sprintf("Backup failed: %s", errorMsg),
		Timestamp: time.Now(),
		Duration:  duration,
		Details: map[string]string{
			"error": errorMsg,
		},
	}
	return m.Notify(ctx, config, event)
}

// NotifyRestoreSuccess sends a restore success notification.
func (m *Manager) NotifyRestoreSuccess(ctx context.Context, config Config, resource, namespace, snapshotID string, duration time.Duration) error {
	event := Event{
		Type:       EventTypeSuccess,
		Resource:   resource,
		Namespace:  namespace,
		Message:    fmt.Sprintf("Restore completed successfully from snapshot: %s", snapshotID),
		Timestamp:  time.Now(),
		Duration:   duration,
		SnapshotID: snapshotID,
	}
	return m.Notify(ctx, config, event)
}

// NotifyRestoreFailure sends a restore failure notification.
func (m *Manager) NotifyRestoreFailure(ctx context.Context, config Config, resource, namespace, errorMsg string, duration time.Duration) error {
	event := Event{
		Type:      EventTypeFailure,
		Resource:  resource,
		Namespace: namespace,
		Message:   fmt.Sprintf("Restore failed: %s", errorMsg),
		Timestamp: time.Now(),
		Duration:  duration,
		Details: map[string]string{
			"error": errorMsg,
		},
	}
	return m.Notify(ctx, config, event)
}
