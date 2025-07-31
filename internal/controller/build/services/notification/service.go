// Copyright 2025 The OpenChoreo Authors
// SPDX-License-Identifier: Apache-2.0

package notification

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	openchoreov1alpha1 "github.com/openchoreo/openchoreo/api/v1alpha1"
	"github.com/openchoreo/openchoreo/internal/controller/build/interfaces"
)

// NotificationChannel represents different notification channels
type NotificationChannel string

const (
	ChannelSlack    NotificationChannel = "slack"
	ChannelEmail    NotificationChannel = "email"
	ChannelWebhook  NotificationChannel = "webhook"
	ChannelEventLog NotificationChannel = "eventlog"
)

// NotificationEvent represents different build events to notify about
type NotificationEvent string

const (
	EventBuildStarted    NotificationEvent = "build.started"
	EventBuildCompleted  NotificationEvent = "build.completed"
	EventBuildFailed     NotificationEvent = "build.failed"
	EventWorkloadCreated NotificationEvent = "workload.created"
	EventWorkloadUpdated NotificationEvent = "workload.updated"
)

// Service handles build notifications
type Service struct {
	client   client.Client
	logger   logr.Logger
	channels map[NotificationChannel]bool // enabled channels
}

// NewService creates a new notification service
func NewService(client client.Client) *Service {
	return &Service{
		client: client,
		logger: log.Log.WithName("notification-service"),
		channels: map[NotificationChannel]bool{
			ChannelEventLog: true, // Always enabled for Kubernetes events
		},
	}
}

// EnableChannel enables a notification channel
func (s *Service) EnableChannel(channel NotificationChannel) {
	s.channels[channel] = true
	s.logger.Info("Enabled notification channel", "channel", string(channel))
}

// DisableChannel disables a notification channel
func (s *Service) DisableChannel(channel NotificationChannel) {
	s.channels[channel] = false
	s.logger.Info("Disabled notification channel", "channel", string(channel))
}

// NotifyBuildStarted sends notification when build starts
func (s *Service) NotifyBuildStarted(ctx context.Context, build *openchoreov1alpha1.Build) error {
	return s.sendNotification(ctx, EventBuildStarted, map[string]interface{}{
		"build":     build.Name,
		"namespace": build.Namespace,
		"project":   build.Spec.Owner.ProjectName,
		"component": build.Spec.Owner.ComponentName,
		"repo":      build.Spec.Repository.URL,
	})
}

// NotifyBuildCompleted sends notification when build completes successfully
func (s *Service) NotifyBuildCompleted(ctx context.Context, build *openchoreov1alpha1.Build, artifacts *interfaces.BuildArtifacts) error {
	data := map[string]interface{}{
		"build":     build.Name,
		"namespace": build.Namespace,
		"project":   build.Spec.Owner.ProjectName,
		"component": build.Spec.Owner.ComponentName,
		"repo":      build.Spec.Repository.URL,
		"status":    "success",
	}

	if artifacts != nil && artifacts.Image != "" {
		data["image"] = artifacts.Image
	}

	return s.sendNotification(ctx, EventBuildCompleted, data)
}

// NotifyBuildFailed sends notification when build fails
func (s *Service) NotifyBuildFailed(ctx context.Context, build *openchoreov1alpha1.Build, err error) error {
	return s.sendNotification(ctx, EventBuildFailed, map[string]interface{}{
		"build":     build.Name,
		"namespace": build.Namespace,
		"project":   build.Spec.Owner.ProjectName,
		"component": build.Spec.Owner.ComponentName,
		"repo":      build.Spec.Repository.URL,
		"status":    "failed",
		"error":     err.Error(),
	})
}

// NotifyWorkloadCreated sends notification when workload is created
func (s *Service) NotifyWorkloadCreated(ctx context.Context, build *openchoreov1alpha1.Build, workloadName string) error {
	return s.sendNotification(ctx, EventWorkloadCreated, map[string]interface{}{
		"build":     build.Name,
		"namespace": build.Namespace,
		"project":   build.Spec.Owner.ProjectName,
		"component": build.Spec.Owner.ComponentName,
		"workload":  workloadName,
	})
}

// NotifyWorkloadUpdated sends notification when workload is updated
func (s *Service) NotifyWorkloadUpdated(ctx context.Context, build *openchoreov1alpha1.Build, workloadName string) error {
	return s.sendNotification(ctx, EventWorkloadUpdated, map[string]interface{}{
		"build":     build.Name,
		"namespace": build.Namespace,
		"project":   build.Spec.Owner.ProjectName,
		"component": build.Spec.Owner.ComponentName,
		"workload":  workloadName,
	})
}

// sendNotification sends notification to all enabled channels
func (s *Service) sendNotification(ctx context.Context, event NotificationEvent, data map[string]interface{}) error {
	logger := s.logger.WithValues("event", string(event))

	var errors []error

	for channel, enabled := range s.channels {
		if !enabled {
			continue
		}

		if err := s.sendToChannel(ctx, channel, event, data); err != nil {
			logger.Error(err, "Failed to send notification", "channel", string(channel))
			errors = append(errors, fmt.Errorf("channel %s: %w", channel, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("notification errors: %v", errors)
	}

	return nil
}

// sendToChannel sends notification to a specific channel
func (s *Service) sendToChannel(ctx context.Context, channel NotificationChannel, event NotificationEvent, data map[string]interface{}) error {
	switch channel {
	case ChannelEventLog:
		return s.sendToEventLog(ctx, event, data)
	case ChannelSlack:
		return s.sendToSlack(ctx, event, data)
	case ChannelEmail:
		return s.sendToEmail(ctx, event, data)
	case ChannelWebhook:
		return s.sendToWebhook(ctx, event, data)
	default:
		return fmt.Errorf("unsupported notification channel: %s", channel)
	}
}

// sendToEventLog creates Kubernetes events
func (s *Service) sendToEventLog(ctx context.Context, event NotificationEvent, data map[string]interface{}) error {
	// TODO: Create Kubernetes Event resources
	s.logger.Info("Build event", "event", string(event), "data", data)
	return nil
}

// sendToSlack sends notification to Slack (placeholder)
func (s *Service) sendToSlack(ctx context.Context, event NotificationEvent, data map[string]interface{}) error {
	// TODO: Implement Slack notification
	s.logger.Info("Would send Slack notification", "event", string(event))
	return nil
}

// sendToEmail sends email notification (placeholder)
func (s *Service) sendToEmail(ctx context.Context, event NotificationEvent, data map[string]interface{}) error {
	// TODO: Implement email notification
	s.logger.Info("Would send email notification", "event", string(event))
	return nil
}

// sendToWebhook sends webhook notification (placeholder)
func (s *Service) sendToWebhook(ctx context.Context, event NotificationEvent, data map[string]interface{}) error {
	// TODO: Implement webhook notification
	s.logger.Info("Would send webhook notification", "event", string(event))
	return nil
}
