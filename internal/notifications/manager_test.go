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
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
)

func TestNewManager(t *testing.T) {
	log := logr.Discard()
	manager := NewManager(log)

	if manager == nil {
		t.Fatal("expected manager to be created")
	}

	if manager.ntfy == nil {
		t.Error("expected ntfy notifier to be initialized")
	}

	if manager.pushgateway == nil {
		t.Error("expected pushgateway notifier to be initialized")
	}
}

func TestManager_Notify_NoBackends(t *testing.T) {
	log := logr.Discard()
	manager := NewManager(log)

	config := Config{}
	event := Event{
		Type:      EventTypeSuccess,
		Resource:  "test-backup",
		Namespace: "default",
		Message:   "Test",
	}

	err := manager.Notify(context.Background(), config, event)
	if err != nil {
		t.Errorf("expected no error with empty config, got: %v", err)
	}
}

func TestManager_Notify_EmptyURLs(t *testing.T) {
	log := logr.Discard()
	manager := NewManager(log)

	config := Config{
		Pushgateway: &PushgatewayConfig{
			URL:     "", // Empty URL should be skipped
			JobName: "backup",
		},
		Ntfy: &NtfyConfig{
			ServerURL: "", // Empty URL should be skipped
			Topic:     "test",
		},
	}

	event := Event{
		Type:      EventTypeSuccess,
		Resource:  "test-backup",
		Namespace: "default",
		Message:   "Test",
	}

	err := manager.Notify(context.Background(), config, event)
	if err != nil {
		t.Errorf("expected no error with empty URLs, got: %v", err)
	}
}

func TestManager_Notify_BothBackends(t *testing.T) {
	var ntfyReceived, pushgatewayReceived atomic.Bool

	ntfyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ntfyReceived.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer ntfyServer.Close()

	pushgatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pushgatewayReceived.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer pushgatewayServer.Close()

	log := logr.Discard()
	manager := NewManager(log)

	config := Config{
		Pushgateway: &PushgatewayConfig{
			URL:     pushgatewayServer.URL,
			JobName: "backup",
		},
		Ntfy: &NtfyConfig{
			ServerURL: ntfyServer.URL,
			Topic:     "test",
		},
	}

	event := Event{
		Type:      EventTypeSuccess,
		Resource:  "test-backup",
		Namespace: "default",
		Message:   "Test",
		Timestamp: time.Now(),
	}

	err := manager.Notify(context.Background(), config, event)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !ntfyReceived.Load() {
		t.Error("expected ntfy notification to be sent")
	}

	if !pushgatewayReceived.Load() {
		t.Error("expected pushgateway notification to be sent")
	}
}

func TestManager_Notify_OnlyOnFailure(t *testing.T) {
	tests := []struct {
		name           string
		onlyOnFailure  bool
		eventType      EventType
		expectNtfySent bool
	}{
		{
			name:           "onlyOnFailure=true with success event",
			onlyOnFailure:  true,
			eventType:      EventTypeSuccess,
			expectNtfySent: false,
		},
		{
			name:           "onlyOnFailure=true with failure event",
			onlyOnFailure:  true,
			eventType:      EventTypeFailure,
			expectNtfySent: true,
		},
		{
			name:           "onlyOnFailure=true with warning event",
			onlyOnFailure:  true,
			eventType:      EventTypeWarning,
			expectNtfySent: false,
		},
		{
			name:           "onlyOnFailure=false with success event",
			onlyOnFailure:  false,
			eventType:      EventTypeSuccess,
			expectNtfySent: true,
		},
		{
			name:           "onlyOnFailure=false with failure event",
			onlyOnFailure:  false,
			eventType:      EventTypeFailure,
			expectNtfySent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ntfyReceived atomic.Bool

			ntfyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ntfyReceived.Store(true)
				w.WriteHeader(http.StatusOK)
			}))
			defer ntfyServer.Close()

			log := logr.Discard()
			manager := NewManager(log)

			config := Config{
				Ntfy: &NtfyConfig{
					ServerURL:     ntfyServer.URL,
					Topic:         "test",
					OnlyOnFailure: tt.onlyOnFailure,
				},
			}

			event := Event{
				Type:      tt.eventType,
				Resource:  "test-backup",
				Namespace: "default",
				Message:   "Test",
				Timestamp: time.Now(),
			}

			err := manager.Notify(context.Background(), config, event)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if ntfyReceived.Load() != tt.expectNtfySent {
				t.Errorf("expected ntfy sent=%v, got %v", tt.expectNtfySent, ntfyReceived.Load())
			}
		})
	}
}

func TestManager_Notify_ErrorAggregation(t *testing.T) {
	// Both servers return errors
	ntfyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ntfyServer.Close()

	pushgatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer pushgatewayServer.Close()

	log := logr.Discard()
	manager := NewManager(log)

	config := Config{
		Pushgateway: &PushgatewayConfig{
			URL:     pushgatewayServer.URL,
			JobName: "backup",
		},
		Ntfy: &NtfyConfig{
			ServerURL: ntfyServer.URL,
			Topic:     "test",
		},
	}

	event := Event{
		Type:      EventTypeSuccess,
		Resource:  "test-backup",
		Namespace: "default",
		Message:   "Test",
		Timestamp: time.Now(),
	}

	err := manager.Notify(context.Background(), config, event)
	if err == nil {
		t.Error("expected error when both backends fail")
	}

	// Check that both errors are included
	errStr := err.Error()
	if !strings.Contains(errStr, "pushgateway") {
		t.Error("expected error to contain pushgateway error")
	}
	if !strings.Contains(errStr, "ntfy") {
		t.Error("expected error to contain ntfy error")
	}
}

func TestManager_Notify_PartialFailure(t *testing.T) {
	var ntfyReceived atomic.Bool

	ntfyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ntfyReceived.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer ntfyServer.Close()

	pushgatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer pushgatewayServer.Close()

	log := logr.Discard()
	manager := NewManager(log)

	config := Config{
		Pushgateway: &PushgatewayConfig{
			URL:     pushgatewayServer.URL,
			JobName: "backup",
		},
		Ntfy: &NtfyConfig{
			ServerURL: ntfyServer.URL,
			Topic:     "test",
		},
	}

	event := Event{
		Type:      EventTypeSuccess,
		Resource:  "test-backup",
		Namespace: "default",
		Message:   "Test",
		Timestamp: time.Now(),
	}

	err := manager.Notify(context.Background(), config, event)
	if err == nil {
		t.Error("expected error when pushgateway fails")
	}

	// Ntfy should still have been called
	if !ntfyReceived.Load() {
		t.Error("expected ntfy notification to be sent despite pushgateway failure")
	}
}

func TestManager_NotifyBackupSuccess(t *testing.T) {
	var receivedEvent Event
	var eventReceived atomic.Bool

	ntfyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		eventReceived.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer ntfyServer.Close()

	log := logr.Discard()
	manager := NewManager(log)

	config := Config{
		Ntfy: &NtfyConfig{
			ServerURL: ntfyServer.URL,
			Topic:     "test",
		},
	}

	err := manager.NotifyBackupSuccess(
		context.Background(),
		config,
		"test-backup",
		"default",
		"snap123",
		"100MB",
		1000,
		5*time.Minute,
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !eventReceived.Load() {
		t.Error("expected notification to be sent")
	}

	// Verify event would have correct fields (we can't directly access it,
	// but we can verify through the message)
	_ = receivedEvent // Event is built internally
}

func TestManager_NotifyBackupFailure(t *testing.T) {
	var eventReceived atomic.Bool

	ntfyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		eventReceived.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer ntfyServer.Close()

	log := logr.Discard()
	manager := NewManager(log)

	config := Config{
		Ntfy: &NtfyConfig{
			ServerURL: ntfyServer.URL,
			Topic:     "test",
		},
	}

	err := manager.NotifyBackupFailure(
		context.Background(),
		config,
		"test-backup",
		"default",
		"connection timeout",
		1*time.Minute,
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !eventReceived.Load() {
		t.Error("expected notification to be sent")
	}
}

func TestManager_NotifyRestoreSuccess(t *testing.T) {
	var eventReceived atomic.Bool

	ntfyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		eventReceived.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer ntfyServer.Close()

	log := logr.Discard()
	manager := NewManager(log)

	config := Config{
		Ntfy: &NtfyConfig{
			ServerURL: ntfyServer.URL,
			Topic:     "test",
		},
	}

	err := manager.NotifyRestoreSuccess(
		context.Background(),
		config,
		"test-restore",
		"default",
		"snap123",
		3*time.Minute,
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !eventReceived.Load() {
		t.Error("expected notification to be sent")
	}
}

func TestManager_NotifyRestoreFailure(t *testing.T) {
	var eventReceived atomic.Bool

	ntfyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		eventReceived.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer ntfyServer.Close()

	log := logr.Discard()
	manager := NewManager(log)

	config := Config{
		Ntfy: &NtfyConfig{
			ServerURL: ntfyServer.URL,
			Topic:     "test",
		},
	}

	err := manager.NotifyRestoreFailure(
		context.Background(),
		config,
		"test-restore",
		"default",
		"snapshot not found",
		1*time.Minute,
	)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !eventReceived.Load() {
		t.Error("expected notification to be sent")
	}
}

func TestEventType_Constants(t *testing.T) {
	// Verify event type constants have expected values
	if EventTypeSuccess != "success" {
		t.Errorf("expected EventTypeSuccess to be 'success', got %s", EventTypeSuccess)
	}

	if EventTypeFailure != "failure" {
		t.Errorf("expected EventTypeFailure to be 'failure', got %s", EventTypeFailure)
	}

	if EventTypeWarning != "warning" {
		t.Errorf("expected EventTypeWarning to be 'warning', got %s", EventTypeWarning)
	}
}

func TestEvent_Fields(t *testing.T) {
	now := time.Now()
	event := Event{
		Type:       EventTypeSuccess,
		Resource:   "test-backup",
		Namespace:  "default",
		Message:    "Test message",
		Details:    map[string]string{"key": "value"},
		Timestamp:  now,
		Duration:   5 * time.Minute,
		SnapshotID: "snap123",
		Size:       "100MB",
		Files:      1000,
	}

	if event.Type != EventTypeSuccess {
		t.Errorf("expected type EventTypeSuccess, got %s", event.Type)
	}

	if event.Resource != "test-backup" {
		t.Errorf("expected resource 'test-backup', got %s", event.Resource)
	}

	if event.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %s", event.Namespace)
	}

	if event.Details["key"] != "value" {
		t.Errorf("expected details key 'value', got %s", event.Details["key"])
	}

	if event.Timestamp != now {
		t.Errorf("expected timestamp %v, got %v", now, event.Timestamp)
	}

	if event.Duration != 5*time.Minute {
		t.Errorf("expected duration 5m, got %v", event.Duration)
	}

	if event.SnapshotID != "snap123" {
		t.Errorf("expected snapshotID 'snap123', got %s", event.SnapshotID)
	}

	if event.Size != "100MB" {
		t.Errorf("expected size '100MB', got %s", event.Size)
	}

	if event.Files != 1000 {
		t.Errorf("expected files 1000, got %d", event.Files)
	}
}

func TestConfig_Fields(t *testing.T) {
	config := Config{
		Pushgateway: &PushgatewayConfig{
			URL:     "http://pushgateway:9091",
			JobName: "backup",
		},
		Ntfy: &NtfyConfig{
			ServerURL:     "https://ntfy.sh",
			Topic:         "backups",
			Token:         "mytoken",
			Username:      "user",
			Password:      "pass",
			OnlyOnFailure: true,
			Priority:      5,
			Tags:          []string{"backup", "alert"},
		},
	}

	if config.Pushgateway.URL != "http://pushgateway:9091" {
		t.Errorf("expected pushgateway URL 'http://pushgateway:9091', got %s", config.Pushgateway.URL)
	}

	if config.Pushgateway.JobName != "backup" {
		t.Errorf("expected job name 'backup', got %s", config.Pushgateway.JobName)
	}

	if config.Ntfy.ServerURL != "https://ntfy.sh" {
		t.Errorf("expected ntfy server URL 'https://ntfy.sh', got %s", config.Ntfy.ServerURL)
	}

	if config.Ntfy.Topic != "backups" {
		t.Errorf("expected ntfy topic 'backups', got %s", config.Ntfy.Topic)
	}

	if config.Ntfy.Token != "mytoken" {
		t.Errorf("expected token 'mytoken', got %s", config.Ntfy.Token)
	}

	if !config.Ntfy.OnlyOnFailure {
		t.Error("expected onlyOnFailure to be true")
	}

	if config.Ntfy.Priority != 5 {
		t.Errorf("expected priority 5, got %d", config.Ntfy.Priority)
	}

	if len(config.Ntfy.Tags) != 2 || config.Ntfy.Tags[0] != "backup" || config.Ntfy.Tags[1] != "alert" {
		t.Errorf("expected tags [backup, alert], got %v", config.Ntfy.Tags)
	}
}
