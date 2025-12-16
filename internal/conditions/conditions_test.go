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

package conditions

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetCondition(t *testing.T) {
	tests := []struct {
		name            string
		initial         []metav1.Condition
		newCondition    metav1.Condition
		expectedLen     int
		expectedStatus  metav1.ConditionStatus
		expectedReason  string
		expectedMessage string
	}{
		{
			name:    "add new condition to empty slice",
			initial: []metav1.Condition{},
			newCondition: metav1.Condition{
				Type:    "Ready",
				Status:  metav1.ConditionTrue,
				Reason:  "TestReason",
				Message: "Test message",
			},
			expectedLen:     1,
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  "TestReason",
			expectedMessage: "Test message",
		},
		{
			name: "update existing condition with same status",
			initial: []metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					Reason:             "OldReason",
					Message:            "Old message",
					LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
				},
			},
			newCondition: metav1.Condition{
				Type:    "Ready",
				Status:  metav1.ConditionTrue,
				Reason:  "NewReason",
				Message: "New message",
			},
			expectedLen:     1,
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  "NewReason",
			expectedMessage: "New message",
		},
		{
			name: "update existing condition with different status",
			initial: []metav1.Condition{
				{
					Type:               "Ready",
					Status:             metav1.ConditionTrue,
					Reason:             "OldReason",
					Message:            "Old message",
					LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
				},
			},
			newCondition: metav1.Condition{
				Type:    "Ready",
				Status:  metav1.ConditionFalse,
				Reason:  "NewReason",
				Message: "New message",
			},
			expectedLen:     1,
			expectedStatus:  metav1.ConditionFalse,
			expectedReason:  "NewReason",
			expectedMessage: "New message",
		},
		{
			name: "add new condition to existing slice",
			initial: []metav1.Condition{
				{
					Type:    "Ready",
					Status:  metav1.ConditionTrue,
					Reason:  "ReadyReason",
					Message: "Ready message",
				},
			},
			newCondition: metav1.Condition{
				Type:    "Progressing",
				Status:  metav1.ConditionTrue,
				Reason:  "ProgressingReason",
				Message: "Progressing message",
			},
			expectedLen:     2,
			expectedStatus:  metav1.ConditionTrue,
			expectedReason:  "ProgressingReason",
			expectedMessage: "Progressing message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conditions := tt.initial
			SetCondition(&conditions, tt.newCondition)

			if len(conditions) != tt.expectedLen {
				t.Errorf("expected %d conditions, got %d", tt.expectedLen, len(conditions))
			}

			cond := GetCondition(conditions, tt.newCondition.Type)
			if cond == nil {
				t.Fatalf("expected condition %s to exist", tt.newCondition.Type)
			}

			if cond.Status != tt.expectedStatus {
				t.Errorf("expected status %v, got %v", tt.expectedStatus, cond.Status)
			}

			if cond.Reason != tt.expectedReason {
				t.Errorf("expected reason %s, got %s", tt.expectedReason, cond.Reason)
			}

			if cond.Message != tt.expectedMessage {
				t.Errorf("expected message %s, got %s", tt.expectedMessage, cond.Message)
			}
		})
	}
}

func TestSetCondition_NilSlice(t *testing.T) {
	// Should not panic on nil slice
	SetCondition(nil, metav1.Condition{
		Type:   "Ready",
		Status: metav1.ConditionTrue,
	})
}

func TestGetCondition(t *testing.T) {
	conditions := []metav1.Condition{
		{
			Type:    "Ready",
			Status:  metav1.ConditionTrue,
			Reason:  "TestReason",
			Message: "Test message",
		},
		{
			Type:    "Progressing",
			Status:  metav1.ConditionFalse,
			Reason:  "NotProgressing",
			Message: "Not progressing",
		},
	}

	tests := []struct {
		name       string
		conditions []metav1.Condition
		condType   string
		expectNil  bool
	}{
		{
			name:       "find existing condition",
			conditions: conditions,
			condType:   "Ready",
			expectNil:  false,
		},
		{
			name:       "find another existing condition",
			conditions: conditions,
			condType:   "Progressing",
			expectNil:  false,
		},
		{
			name:       "condition not found",
			conditions: conditions,
			condType:   "NonExistent",
			expectNil:  true,
		},
		{
			name:       "empty conditions",
			conditions: []metav1.Condition{},
			condType:   "Ready",
			expectNil:  true,
		},
		{
			name:       "nil conditions",
			conditions: nil,
			condType:   "Ready",
			expectNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := GetCondition(tt.conditions, tt.condType)
			if tt.expectNil && cond != nil {
				t.Errorf("expected nil, got %v", cond)
			}
			if !tt.expectNil && cond == nil {
				t.Errorf("expected condition, got nil")
			}
		})
	}
}

func TestIsConditionTrue(t *testing.T) {
	conditions := []metav1.Condition{
		{
			Type:   "Ready",
			Status: metav1.ConditionTrue,
		},
		{
			Type:   "Progressing",
			Status: metav1.ConditionFalse,
		},
		{
			Type:   "Unknown",
			Status: metav1.ConditionUnknown,
		},
	}

	tests := []struct {
		name     string
		condType string
		expected bool
	}{
		{"true condition", "Ready", true},
		{"false condition", "Progressing", false},
		{"unknown condition", "Unknown", false},
		{"non-existent condition", "NonExistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsConditionTrue(conditions, tt.condType)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsConditionFalse(t *testing.T) {
	conditions := []metav1.Condition{
		{
			Type:   "Ready",
			Status: metav1.ConditionTrue,
		},
		{
			Type:   "Progressing",
			Status: metav1.ConditionFalse,
		},
		{
			Type:   "Unknown",
			Status: metav1.ConditionUnknown,
		},
	}

	tests := []struct {
		name     string
		condType string
		expected bool
	}{
		{"true condition", "Ready", false},
		{"false condition", "Progressing", true},
		{"unknown condition", "Unknown", false},
		{"non-existent condition", "NonExistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsConditionFalse(conditions, tt.condType)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsConditionUnknown(t *testing.T) {
	conditions := []metav1.Condition{
		{
			Type:   "Ready",
			Status: metav1.ConditionTrue,
		},
		{
			Type:   "Progressing",
			Status: metav1.ConditionFalse,
		},
		{
			Type:   "Unknown",
			Status: metav1.ConditionUnknown,
		},
	}

	tests := []struct {
		name     string
		condType string
		expected bool
	}{
		{"true condition", "Ready", false},
		{"false condition", "Progressing", false},
		{"unknown condition", "Unknown", true},
		{"non-existent condition", "NonExistent", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsConditionUnknown(conditions, tt.condType)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestRemoveCondition(t *testing.T) {
	tests := []struct {
		name         string
		initial      []metav1.Condition
		removeType   string
		expectedLen  int
		shouldRemove bool
	}{
		{
			name: "remove existing condition",
			initial: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue},
				{Type: "Progressing", Status: metav1.ConditionFalse},
			},
			removeType:   "Ready",
			expectedLen:  1,
			shouldRemove: true,
		},
		{
			name: "remove non-existent condition",
			initial: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue},
			},
			removeType:   "NonExistent",
			expectedLen:  1,
			shouldRemove: false,
		},
		{
			name:         "remove from empty slice",
			initial:      []metav1.Condition{},
			removeType:   "Ready",
			expectedLen:  0,
			shouldRemove: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conditions := tt.initial
			RemoveCondition(&conditions, tt.removeType)

			if len(conditions) != tt.expectedLen {
				t.Errorf("expected %d conditions, got %d", tt.expectedLen, len(conditions))
			}

			if tt.shouldRemove {
				cond := GetCondition(conditions, tt.removeType)
				if cond != nil {
					t.Errorf("condition %s should have been removed", tt.removeType)
				}
			}
		})
	}
}

func TestRemoveCondition_NilSlice(t *testing.T) {
	// Should not panic on nil slice
	RemoveCondition(nil, "Ready")
}

func TestNewCondition(t *testing.T) {
	cond := NewCondition("Ready", metav1.ConditionTrue, "TestReason", "Test message")

	if cond.Type != "Ready" {
		t.Errorf("expected type Ready, got %s", cond.Type)
	}

	if cond.Status != metav1.ConditionTrue {
		t.Errorf("expected status True, got %v", cond.Status)
	}

	if cond.Reason != "TestReason" {
		t.Errorf("expected reason TestReason, got %s", cond.Reason)
	}

	if cond.Message != "Test message" {
		t.Errorf("expected message 'Test message', got %s", cond.Message)
	}

	if cond.LastTransitionTime.IsZero() {
		t.Error("expected LastTransitionTime to be set")
	}
}

func TestReadyCondition(t *testing.T) {
	cond := ReadyCondition("TestReason", "Test message")

	if cond.Type != "Ready" {
		t.Errorf("expected type Ready, got %s", cond.Type)
	}

	if cond.Status != metav1.ConditionTrue {
		t.Errorf("expected status True, got %v", cond.Status)
	}

	if cond.Reason != "TestReason" {
		t.Errorf("expected reason TestReason, got %s", cond.Reason)
	}

	if cond.Message != "Test message" {
		t.Errorf("expected message 'Test message', got %s", cond.Message)
	}
}

func TestNotReadyCondition(t *testing.T) {
	cond := NotReadyCondition("TestReason", "Test message")

	if cond.Type != "Ready" {
		t.Errorf("expected type Ready, got %s", cond.Type)
	}

	if cond.Status != metav1.ConditionFalse {
		t.Errorf("expected status False, got %v", cond.Status)
	}

	if cond.Reason != "TestReason" {
		t.Errorf("expected reason TestReason, got %s", cond.Reason)
	}

	if cond.Message != "Test message" {
		t.Errorf("expected message 'Test message', got %s", cond.Message)
	}
}

func TestUnknownCondition(t *testing.T) {
	cond := UnknownCondition("TestReason", "Test message")

	if cond.Type != "Ready" {
		t.Errorf("expected type Ready, got %s", cond.Type)
	}

	if cond.Status != metav1.ConditionUnknown {
		t.Errorf("expected status Unknown, got %v", cond.Status)
	}

	if cond.Reason != "TestReason" {
		t.Errorf("expected reason TestReason, got %s", cond.Reason)
	}

	if cond.Message != "Test message" {
		t.Errorf("expected message 'Test message', got %s", cond.Message)
	}
}

func TestSetCondition_PreservesLastTransitionTime(t *testing.T) {
	oldTime := metav1.NewTime(time.Now().Add(-1 * time.Hour))

	conditions := []metav1.Condition{
		{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "OldReason",
			Message:            "Old message",
			LastTransitionTime: oldTime,
		},
	}

	// Update with same status - should preserve LastTransitionTime
	SetCondition(&conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "NewReason",
		Message: "New message",
	})

	cond := GetCondition(conditions, "Ready")
	if cond.LastTransitionTime != oldTime {
		t.Errorf("expected LastTransitionTime to be preserved, got %v", cond.LastTransitionTime)
	}

	// Update with different status - should update LastTransitionTime
	SetCondition(&conditions, metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionFalse,
		Reason:  "FailedReason",
		Message: "Failed message",
	})

	cond = GetCondition(conditions, "Ready")
	if cond.LastTransitionTime == oldTime {
		t.Error("expected LastTransitionTime to be updated")
	}
}
