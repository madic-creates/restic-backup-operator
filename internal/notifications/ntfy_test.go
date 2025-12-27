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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-logr/logr"
)

func TestNewNtfyNotifier(t *testing.T) {
	log := logr.Discard()
	notifier := NewNtfyNotifier(log)

	if notifier == nil {
		t.Fatal("expected notifier to be created")
	}

	if notifier.httpClient == nil {
		t.Error("expected httpClient to be initialized")
	}

	if notifier.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", notifier.httpClient.Timeout)
	}
}

func TestNtfyNotifier_Notify(t *testing.T) {
	tests := []struct {
		name           string
		config         NtfyConfig
		event          Event
		serverStatus   int
		expectedTitle  string
		expectedTags   []string
		expectPriority int
		expectError    bool
	}{
		{
			name: "success event with defaults",
			config: NtfyConfig{
				ServerURL: "", // Will be set by test server
				Topic:     "test-topic",
			},
			event: Event{
				Type:       EventTypeSuccess,
				Resource:   "test-backup",
				Namespace:  "default",
				Message:    "Backup completed",
				Timestamp:  time.Now(),
				Duration:   5 * time.Minute,
				SnapshotID: "abc123",
				Size:       "100MB",
				Files:      1000,
			},
			serverStatus:   http.StatusOK,
			expectedTitle:  "default/test-backup - Backup Succeeded",
			expectedTags:   []string{"white_check_mark"},
			expectPriority: 3,
			expectError:    false,
		},
		{
			name: "failure event with defaults",
			config: NtfyConfig{
				ServerURL: "",
				Topic:     "test-topic",
			},
			event: Event{
				Type:      EventTypeFailure,
				Resource:  "test-backup",
				Namespace: "production",
				Message:   "Backup failed: connection timeout",
				Timestamp: time.Now(),
				Duration:  1 * time.Minute,
			},
			serverStatus:   http.StatusOK,
			expectedTitle:  "production/test-backup - Backup Failed",
			expectedTags:   []string{"x"},
			expectPriority: 5,
			expectError:    false,
		},
		{
			name: "warning event with defaults",
			config: NtfyConfig{
				ServerURL: "",
				Topic:     "test-topic",
			},
			event: Event{
				Type:      EventTypeWarning,
				Resource:  "test-backup",
				Namespace: "staging",
				Message:   "Backup completed with warnings",
				Timestamp: time.Now(),
			},
			serverStatus:   http.StatusOK,
			expectedTitle:  "staging/test-backup - Backup Warning",
			expectedTags:   []string{"warning"},
			expectPriority: 4,
			expectError:    false,
		},
		{
			name: "custom tags and priority",
			config: NtfyConfig{
				ServerURL: "",
				Topic:     "custom-topic",
				Priority:  2,
				Tags:      []string{"custom-tag", "backup"},
			},
			event: Event{
				Type:      EventTypeSuccess,
				Resource:  "test-backup",
				Namespace: "default",
				Message:   "Backup completed",
				Timestamp: time.Now(),
			},
			serverStatus:   http.StatusOK,
			expectedTitle:  "default/test-backup - Backup Succeeded",
			expectedTags:   []string{"custom-tag", "backup"},
			expectPriority: 2,
			expectError:    false,
		},
		{
			name: "with bearer token",
			config: NtfyConfig{
				ServerURL: "",
				Topic:     "test-topic",
				Token:     "test-token",
			},
			event: Event{
				Type:      EventTypeSuccess,
				Resource:  "test-backup",
				Namespace: "default",
				Message:   "Backup completed",
				Timestamp: time.Now(),
			},
			serverStatus:   http.StatusOK,
			expectedTitle:  "default/test-backup - Backup Succeeded",
			expectedTags:   []string{"white_check_mark"},
			expectPriority: 3,
			expectError:    false,
		},
		{
			name: "with basic auth",
			config: NtfyConfig{
				ServerURL: "",
				Topic:     "test-topic",
				Username:  "testuser",
				Password:  "testpass",
			},
			event: Event{
				Type:      EventTypeSuccess,
				Resource:  "test-backup",
				Namespace: "default",
				Message:   "Backup completed",
				Timestamp: time.Now(),
			},
			serverStatus:   http.StatusOK,
			expectedTitle:  "default/test-backup - Backup Succeeded",
			expectedTags:   []string{"white_check_mark"},
			expectPriority: 3,
			expectError:    false,
		},
		{
			name: "server error",
			config: NtfyConfig{
				ServerURL: "",
				Topic:     "test-topic",
			},
			event: Event{
				Type:      EventTypeSuccess,
				Resource:  "test-backup",
				Namespace: "default",
				Message:   "Backup completed",
			},
			serverStatus: http.StatusInternalServerError,
			expectError:  true,
		},
		{
			name: "server unauthorized",
			config: NtfyConfig{
				ServerURL: "",
				Topic:     "test-topic",
			},
			event: Event{
				Type:      EventTypeSuccess,
				Resource:  "test-backup",
				Namespace: "default",
				Message:   "Backup completed",
			},
			serverStatus: http.StatusUnauthorized,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedRequest *ntfyMessage
			var receivedAuthHeader string

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedAuthHeader = r.Header.Get("Authorization")

				if r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
				}

				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatalf("failed to read request body: %v", err)
				}

				var msg ntfyMessage
				if err := json.Unmarshal(body, &msg); err != nil {
					t.Fatalf("failed to unmarshal request body: %v", err)
				}
				receivedRequest = &msg

				w.WriteHeader(tt.serverStatus)
			}))
			defer server.Close()

			tt.config.ServerURL = server.URL

			log := logr.Discard()
			notifier := NewNtfyNotifier(log)

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

			if receivedRequest == nil {
				t.Fatal("expected request to be received")
			}

			if receivedRequest.Topic != tt.config.Topic {
				t.Errorf("expected topic %s, got %s", tt.config.Topic, receivedRequest.Topic)
			}

			if receivedRequest.Title != tt.expectedTitle {
				t.Errorf("expected title %s, got %s", tt.expectedTitle, receivedRequest.Title)
			}

			if receivedRequest.Priority != tt.expectPriority {
				t.Errorf("expected priority %d, got %d", tt.expectPriority, receivedRequest.Priority)
			}

			if len(receivedRequest.Tags) != len(tt.expectedTags) {
				t.Errorf("expected %d tags, got %d", len(tt.expectedTags), len(receivedRequest.Tags))
			} else {
				for i, tag := range tt.expectedTags {
					if receivedRequest.Tags[i] != tag {
						t.Errorf("expected tag[%d] %s, got %s", i, tag, receivedRequest.Tags[i])
					}
				}
			}

			// Verify authentication header
			if tt.config.Token != "" {
				expectedAuth := "Bearer " + tt.config.Token
				if receivedAuthHeader != expectedAuth {
					t.Errorf("expected auth header %s, got %s", expectedAuth, receivedAuthHeader)
				}
			} else if tt.config.Username != "" && tt.config.Password != "" {
				if receivedAuthHeader == "" || receivedAuthHeader[:6] != "Basic " {
					t.Errorf("expected Basic auth header, got %s", receivedAuthHeader)
				}
			}
		})
	}
}

func TestNtfyNotifier_Notify_MessageContent(t *testing.T) {
	var receivedMessage string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var msg ntfyMessage
		_ = json.Unmarshal(body, &msg)
		receivedMessage = msg.Message
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	log := logr.Discard()
	notifier := NewNtfyNotifier(log)

	event := Event{
		Type:      EventTypeSuccess,
		Resource:  "test-backup",
		Namespace: "default",
		Message:   "Backup completed",
		Duration:  5*time.Minute + 30*time.Second,
		Size:      "100MB",
		Files:     1234,
	}

	config := NtfyConfig{
		ServerURL: server.URL,
		Topic:     "test",
	}

	err := notifier.Notify(context.Background(), config, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check message contains expected information
	expectedParts := []string{
		"Backup completed",
		"Duration: 5m30s",
		"Size: 100MB",
		"Files: 1234",
	}

	for _, part := range expectedParts {
		if !containsString(receivedMessage, part) {
			t.Errorf("expected message to contain %q, got %q", part, receivedMessage)
		}
	}
}

func TestNtfyNotifier_Notify_URLTrailingSlash(t *testing.T) {
	var receivedURL string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedURL = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	log := logr.Discard()
	notifier := NewNtfyNotifier(log)

	config := NtfyConfig{
		ServerURL: server.URL + "/", // URL with trailing slash
		Topic:     "test",
	}

	event := Event{
		Type:      EventTypeSuccess,
		Resource:  "test-backup",
		Namespace: "default",
		Message:   "Test",
	}

	err := notifier.Notify(context.Background(), config, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The trailing slash should be trimmed
	if receivedURL != "/" {
		t.Errorf("expected path /, got %s", receivedURL)
	}
}

func TestNtfyNotifier_Notify_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	log := logr.Discard()
	notifier := NewNtfyNotifier(log)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	config := NtfyConfig{
		ServerURL: server.URL,
		Topic:     "test",
	}

	event := Event{
		Type:      EventTypeSuccess,
		Resource:  "test-backup",
		Namespace: "default",
		Message:   "Test",
	}

	err := notifier.Notify(ctx, config, event)
	if err == nil {
		t.Error("expected error due to context cancellation")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
