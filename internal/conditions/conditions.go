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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetCondition updates or adds a condition to the slice.
// If a condition with the same type exists, it updates it.
// Otherwise, it appends the new condition.
func SetCondition(conditions *[]metav1.Condition, condition metav1.Condition) {
	if conditions == nil {
		return
	}

	now := metav1.NewTime(time.Now())
	condition.LastTransitionTime = now

	for i, c := range *conditions {
		if c.Type == condition.Type {
			// Only update LastTransitionTime if status changed
			if c.Status == condition.Status {
				condition.LastTransitionTime = c.LastTransitionTime
			}
			(*conditions)[i] = condition
			return
		}
	}

	*conditions = append(*conditions, condition)
}

// GetCondition returns the condition with the given type, or nil if not found.
func GetCondition(conditions []metav1.Condition, condType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == condType {
			return &conditions[i]
		}
	}
	return nil
}

// IsConditionTrue returns true if the condition with the given type has status True.
func IsConditionTrue(conditions []metav1.Condition, condType string) bool {
	cond := GetCondition(conditions, condType)
	return cond != nil && cond.Status == metav1.ConditionTrue
}

// IsConditionFalse returns true if the condition with the given type has status False.
func IsConditionFalse(conditions []metav1.Condition, condType string) bool {
	cond := GetCondition(conditions, condType)
	return cond != nil && cond.Status == metav1.ConditionFalse
}

// IsConditionUnknown returns true if the condition with the given type has status Unknown.
func IsConditionUnknown(conditions []metav1.Condition, condType string) bool {
	cond := GetCondition(conditions, condType)
	return cond != nil && cond.Status == metav1.ConditionUnknown
}

// RemoveCondition removes the condition with the given type from the slice.
func RemoveCondition(conditions *[]metav1.Condition, condType string) {
	if conditions == nil {
		return
	}

	newConditions := make([]metav1.Condition, 0, len(*conditions))
	for _, c := range *conditions {
		if c.Type != condType {
			newConditions = append(newConditions, c)
		}
	}
	*conditions = newConditions
}

// NewCondition creates a new condition with the given parameters.
func NewCondition(condType string, status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.NewTime(time.Now()),
	}
}

// ReadyCondition creates a Ready condition with status True.
func ReadyCondition(reason, message string) metav1.Condition {
	return NewCondition("Ready", metav1.ConditionTrue, reason, message)
}

// NotReadyCondition creates a Ready condition with status False.
func NotReadyCondition(reason, message string) metav1.Condition {
	return NewCondition("Ready", metav1.ConditionFalse, reason, message)
}

// UnknownCondition creates a Ready condition with status Unknown.
func UnknownCondition(reason, message string) metav1.Condition {
	return NewCondition("Ready", metav1.ConditionUnknown, reason, message)
}
