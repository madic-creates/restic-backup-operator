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

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

// PushgatewayNotifier sends metrics to Prometheus Pushgateway.
type PushgatewayNotifier struct {
	log logr.Logger
}

// NewPushgatewayNotifier creates a new Pushgateway notifier.
func NewPushgatewayNotifier(log logr.Logger) *PushgatewayNotifier {
	return &PushgatewayNotifier{
		log: log,
	}
}

// Notify sends metrics to Pushgateway.
func (p *PushgatewayNotifier) Notify(ctx context.Context, config PushgatewayConfig, event Event) error {
	jobName := config.JobName
	if jobName == "" {
		jobName = "backup"
	}

	// Create metrics
	registry := prometheus.NewRegistry()

	// Backup duration
	durationGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "backup_duration_seconds",
		Help: "Duration of the backup operation in seconds",
	})
	durationGauge.Set(event.Duration.Seconds())
	registry.MustRegister(durationGauge)

	// Backup timestamp
	timestampGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "backup_start_timestamp",
		Help: "Unix timestamp when the backup started",
	})
	timestampGauge.Set(float64(event.Timestamp.Unix()))
	registry.MustRegister(timestampGauge)

	// Backup status (1 = success, 0 = failure)
	statusGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "backup_status",
		Help: "Status of the backup (1 = success, 0 = failure)",
	})
	if event.Type == EventTypeSuccess {
		statusGauge.Set(1)
	} else {
		statusGauge.Set(0)
	}
	registry.MustRegister(statusGauge)

	// Backup files count
	if event.Files > 0 {
		filesGauge := prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "backup_snapshot_files_total",
			Help: "Number of files in the backup snapshot",
		})
		filesGauge.Set(float64(event.Files))
		registry.MustRegister(filesGauge)
	}

	// Push to Pushgateway
	pusher := push.New(config.URL, jobName).
		Grouping("backup", event.Resource).
		Grouping("namespace", event.Namespace).
		Gatherer(registry)

	if err := pusher.Push(); err != nil {
		return fmt.Errorf("failed to push metrics to Pushgateway: %w", err)
	}

	p.log.V(1).Info("Pushed metrics to Pushgateway",
		"url", config.URL,
		"job", jobName,
		"backup", event.Resource,
		"status", event.Type)

	return nil
}
