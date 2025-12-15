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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-logr/logr"
)

// NtfyNotifier sends notifications via ntfy.
type NtfyNotifier struct {
	log        logr.Logger
	httpClient *http.Client
}

// NewNtfyNotifier creates a new ntfy notifier.
func NewNtfyNotifier(log logr.Logger) *NtfyNotifier {
	return &NtfyNotifier{
		log: log,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ntfyMessage represents a ntfy notification message.
type ntfyMessage struct {
	Topic    string   `json:"topic"`
	Message  string   `json:"message"`
	Title    string   `json:"title,omitempty"`
	Priority int      `json:"priority,omitempty"`
	Tags     []string `json:"tags,omitempty"`
}

// Notify sends a notification via ntfy.
func (n *NtfyNotifier) Notify(ctx context.Context, config NtfyConfig, event Event) error {
	// Build the title
	title := fmt.Sprintf("%s/%s - Backup", event.Namespace, event.Resource)
	switch event.Type {
	case EventTypeSuccess:
		title += " Succeeded"
	case EventTypeFailure:
		title += " Failed"
	case EventTypeWarning:
		title += " Warning"
	}

	// Build tags
	tags := config.Tags
	if len(tags) == 0 {
		switch event.Type {
		case EventTypeSuccess:
			tags = []string{"white_check_mark"}
		case EventTypeFailure:
			tags = []string{"x"}
		case EventTypeWarning:
			tags = []string{"warning"}
		}
	}

	// Build the message
	var msgBuilder strings.Builder
	msgBuilder.WriteString(event.Message)
	if event.Duration > 0 {
		msgBuilder.WriteString(fmt.Sprintf("\nDuration: %s", event.Duration.Round(time.Second)))
	}
	if event.Size != "" {
		msgBuilder.WriteString(fmt.Sprintf("\nSize: %s", event.Size))
	}
	if event.Files > 0 {
		msgBuilder.WriteString(fmt.Sprintf("\nFiles: %d", event.Files))
	}

	// Set priority
	priority := int(config.Priority)
	if priority == 0 {
		switch event.Type {
		case EventTypeSuccess:
			priority = 3
		case EventTypeFailure:
			priority = 5
		case EventTypeWarning:
			priority = 4
		}
	}

	msg := ntfyMessage{
		Topic:    config.Topic,
		Title:    title,
		Message:  msgBuilder.String(),
		Priority: priority,
		Tags:     tags,
	}

	// Marshal to JSON
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal ntfy message: %w", err)
	}

	// Build URL
	url := strings.TrimSuffix(config.ServerURL, "/")

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create ntfy request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Add authorization if provided
	if config.AuthHeader != "" {
		req.Header.Set("Authorization", config.AuthHeader)
	}

	// Send request
	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send ntfy notification: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned status code %d", resp.StatusCode)
	}

	n.log.V(1).Info("Sent ntfy notification", "topic", config.Topic, "type", event.Type)

	return nil
}
