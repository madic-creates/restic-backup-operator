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
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-logr/logr"
)

func TestNewPushgatewayNotifier(t *testing.T) {
	log := logr.Discard()
	notifier := NewPushgatewayNotifier(log)

	if notifier == nil {
		t.Fatal("expected notifier to be created")
	}
}

func TestPushgatewayNotifier_Notify(t *testing.T) {
	tests := []struct {
		name            string
		config          PushgatewayConfig
		event           Event
		serverStatus    int
		expectError     bool
		expectedJobName string
		expectMetrics   []string
	}{
		{
			name: "success event with default job name",
			config: PushgatewayConfig{
				URL:     "",
				JobName: "",
			},
			event: Event{
				Type:       EventTypeSuccess,
				Resource:   "test-backup",
				Namespace:  "default",
				Message:    "Backup completed",
				Timestamp:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Duration:   5 * time.Minute,
				SnapshotID: "abc123",
				Size:       "100MB",
				Files:      1000,
			},
			serverStatus:    http.StatusOK,
			expectError:     false,
			expectedJobName: "backup",
			expectMetrics: []string{
				"backup_duration_seconds",
				"backup_start_timestamp",
				"backup_status",
				"backup_snapshot_files_total",
			},
		},
		{
			name: "failure event with custom job name",
			config: PushgatewayConfig{
				URL:     "",
				JobName: "custom-backup-job",
			},
			event: Event{
				Type:      EventTypeFailure,
				Resource:  "test-backup",
				Namespace: "production",
				Message:   "Backup failed",
				Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Duration:  1 * time.Minute,
			},
			serverStatus:    http.StatusOK,
			expectError:     false,
			expectedJobName: "custom-backup-job",
			expectMetrics: []string{
				"backup_duration_seconds",
				"backup_start_timestamp",
				"backup_status",
			},
		},
		{
			name: "success event without files",
			config: PushgatewayConfig{
				URL:     "",
				JobName: "backup",
			},
			event: Event{
				Type:      EventTypeSuccess,
				Resource:  "test-backup",
				Namespace: "default",
				Message:   "Backup completed",
				Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Duration:  2 * time.Minute,
				Files:     0, // No files
			},
			serverStatus:    http.StatusOK,
			expectError:     false,
			expectedJobName: "backup",
			expectMetrics: []string{
				"backup_duration_seconds",
				"backup_start_timestamp",
				"backup_status",
			},
		},
		{
			name: "server error",
			config: PushgatewayConfig{
				URL:     "",
				JobName: "backup",
			},
			event: Event{
				Type:      EventTypeSuccess,
				Resource:  "test-backup",
				Namespace: "default",
				Message:   "Backup completed",
				Timestamp: time.Now(),
			},
			serverStatus: http.StatusInternalServerError,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedBody string
			var receivedPath string
			var receivedMethod string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				receivedMethod = r.Method
				body, _ := io.ReadAll(r.Body)
				receivedBody = string(body)
				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			tt.config.URL = server.URL

			log := logr.Discard()
			notifier := NewPushgatewayNotifier(log)

			err := notifier.Notify(context.Background(), tt.config, tt.event)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify HTTP method is PUT (default for pushgateway)
			if receivedMethod != http.MethodPut {
				t.Errorf("expected method PUT, got %s", receivedMethod)
			}

			// Verify job name is in the path
			if !strings.Contains(receivedPath, tt.expectedJobName) {
				t.Errorf("expected path to contain job name %q, got %q", tt.expectedJobName, receivedPath)
			}

			// Verify backup grouping is in the path
			if !strings.Contains(receivedPath, tt.event.Resource) {
				t.Errorf("expected path to contain resource %q, got %q", tt.event.Resource, receivedPath)
			}

			// Verify namespace grouping is in the path
			if !strings.Contains(receivedPath, tt.event.Namespace) {
				t.Errorf("expected path to contain namespace %q, got %q", tt.event.Namespace, receivedPath)
			}

			// Verify expected metrics are present in the body (metric names are in protobuf)
			for _, metric := range tt.expectMetrics {
				if !strings.Contains(receivedBody, metric) {
					t.Errorf("expected body to contain metric %q", metric)
				}
			}

			// Verify files metric is not present when Files is 0
			if tt.event.Files == 0 && strings.Contains(receivedBody, "backup_snapshot_files_total") {
				t.Error("expected files metric to not be present when Files is 0")
			}
		})
	}
}

func TestPushgatewayNotifier_Notify_MetricsPresent(t *testing.T) {
	var receivedBody string
	var requestReceived atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived.Store(true)
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	log := logr.Discard()
	notifier := NewPushgatewayNotifier(log)

	event := Event{
		Type:      EventTypeSuccess,
		Resource:  "test-backup",
		Namespace: "default",
		Message:   "Backup completed",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Duration:  5*time.Minute + 30*time.Second,
		Files:     1234,
	}

	config := PushgatewayConfig{
		URL:     server.URL,
		JobName: "backup",
	}

	err := notifier.Notify(context.Background(), config, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !requestReceived.Load() {
		t.Fatal("expected request to be received")
	}

	// Verify all expected metric names are present
	expectedMetrics := []string{
		"backup_duration_seconds",
		"backup_start_timestamp",
		"backup_status",
		"backup_snapshot_files_total",
	}

	for _, metric := range expectedMetrics {
		if !strings.Contains(receivedBody, metric) {
			t.Errorf("expected body to contain metric %q", metric)
		}
	}

	// Verify metric descriptions are present
	expectedDescriptions := []string{
		"Duration of the backup operation in seconds",
		"Unix timestamp when the backup started",
		"Status of the backup",
		"Number of files in the backup snapshot",
	}

	for _, desc := range expectedDescriptions {
		if !strings.Contains(receivedBody, desc) {
			t.Errorf("expected body to contain description %q", desc)
		}
	}
}

func TestPushgatewayNotifier_Notify_WarningEvent(t *testing.T) {
	var requestReceived atomic.Bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	log := logr.Discard()
	notifier := NewPushgatewayNotifier(log)

	event := Event{
		Type:      EventTypeWarning,
		Resource:  "test-backup",
		Namespace: "default",
		Message:   "Backup completed with warnings",
		Timestamp: time.Now(),
	}

	config := PushgatewayConfig{
		URL:     server.URL,
		JobName: "backup",
	}

	err := notifier.Notify(context.Background(), config, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !requestReceived.Load() {
		t.Error("expected request to be received for warning event")
	}
}

func TestPushgatewayNotifier_Notify_ConnectionError(t *testing.T) {
	log := logr.Discard()
	notifier := NewPushgatewayNotifier(log)

	config := PushgatewayConfig{
		URL:     "http://localhost:99999", // Invalid port
		JobName: "backup",
	}

	event := Event{
		Type:      EventTypeSuccess,
		Resource:  "test-backup",
		Namespace: "default",
		Message:   "Backup completed",
		Timestamp: time.Now(),
	}

	err := notifier.Notify(context.Background(), config, event)
	if err == nil {
		t.Error("expected error for connection failure")
	}
}

func TestPushgatewayNotifier_Notify_URLPath(t *testing.T) {
	tests := []struct {
		name          string
		jobName       string
		resource      string
		namespace     string
		expectedParts []string
	}{
		{
			name:      "standard path",
			jobName:   "backup",
			resource:  "my-backup",
			namespace: "default",
			expectedParts: []string{
				"/metrics/job/backup",
				"backup/my-backup",
				"namespace/default",
			},
		},
		{
			name:      "special characters in resource",
			jobName:   "backup",
			resource:  "backup-with-dashes",
			namespace: "kube-system",
			expectedParts: []string{
				"/metrics/job/backup",
				"backup/backup-with-dashes",
				"namespace/kube-system",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedPath string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedPath = r.URL.Path
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			log := logr.Discard()
			notifier := NewPushgatewayNotifier(log)

			config := PushgatewayConfig{
				URL:     server.URL,
				JobName: tt.jobName,
			}

			event := Event{
				Type:      EventTypeSuccess,
				Resource:  tt.resource,
				Namespace: tt.namespace,
				Message:   "Test",
				Timestamp: time.Now(),
			}

			err := notifier.Notify(context.Background(), config, event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, part := range tt.expectedParts {
				if !strings.Contains(receivedPath, part) {
					t.Errorf("expected path to contain %q, got %q", part, receivedPath)
				}
			}
		})
	}
}

func TestPushgatewayNotifier_Notify_FilesMetricConditional(t *testing.T) {
	tests := []struct {
		name        string
		files       int64
		expectFiles bool
	}{
		{
			name:        "with files",
			files:       100,
			expectFiles: true,
		},
		{
			name:        "without files",
			files:       0,
			expectFiles: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedBody string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				receivedBody = string(body)
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			log := logr.Discard()
			notifier := NewPushgatewayNotifier(log)

			config := PushgatewayConfig{
				URL:     server.URL,
				JobName: "backup",
			}

			event := Event{
				Type:      EventTypeSuccess,
				Resource:  "test-backup",
				Namespace: "default",
				Message:   "Test",
				Timestamp: time.Now(),
				Files:     tt.files,
			}

			err := notifier.Notify(context.Background(), config, event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			hasFilesMetric := strings.Contains(receivedBody, "backup_snapshot_files_total")
			if hasFilesMetric != tt.expectFiles {
				t.Errorf("expected files metric present=%v, got %v", tt.expectFiles, hasFilesMetric)
			}
		})
	}
}
