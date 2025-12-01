// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package auditlogger

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/admin/back-office/changehistory"
	"storj.io/storj/satellite/analytics"
)

// Config contains configuration for the audit logger.
type Config struct {
	Enabled bool `help:"enable audit logging for admin operations" default:"false"`
	Caps    CapsConfig
}

// Event represents an audit event for an admin operation.
type Event struct {
	UserID     uuid.UUID              // The user who owns the affected item
	ProjectID  *uuid.UUID             // The project related to the affected item (if applicable)
	BucketName *string                // The bucket related to the affected item (if applicable)
	Action     string                 // e.g., "update_user", "freeze_account"
	AdminEmail string                 // Who performed the action
	ItemType   changehistory.ItemType // e.g., "User", "Project", "Bucket"
	Reason     string                 // Why the change was made
	Before     any                    // Previous values
	After      any                    // New values
	Timestamp  time.Time
}

// Logger implements Logger using Segment analytics service.
type Logger struct {
	log       *zap.Logger
	analytics *analytics.Service

	changeHistory changehistory.DB
	changeEvents  chan Event
	worker        sync2.Limiter

	config Config
}

// New creates a new Logger.
func New(log *zap.Logger, analytics *analytics.Service, changeHistory changehistory.DB, config Config) *Logger {
	return &Logger{
		log:       log,
		analytics: analytics,

		changeHistory: changeHistory,
		changeEvents:  make(chan Event, 100),
		worker:        *sync2.NewLimiter(2),

		config: config,
	}
}

// EnqueueChangeEvent logs an admin change event to Segment.
func (s *Logger) EnqueueChangeEvent(event Event) {
	if !s.config.Enabled {
		return
	}

	s.changeEvents <- event
}

func (s *Logger) logChangeEvent(ctx context.Context, event Event) {
	if !s.config.Enabled {
		return
	}

	var itemID string
	if event.ItemType == changehistory.ItemTypeUser {
		itemID = event.UserID.String()
	} else if event.ItemType == changehistory.ItemTypeProject && event.ProjectID != nil {
		itemID = event.ProjectID.String()
	} else if event.ItemType == changehistory.ItemTypeBucket && event.BucketName != nil {
		itemID = *event.BucketName
	} else {
		s.log.Error("logging audit event with missing item ID")
	}

	changes := BuildChangeSet(event.Before, event.After, s.config.Caps)

	s.analytics.TrackAdminAuditEvent(event.UserID, map[string]any{
		"action":      event.Action,
		"admin_email": event.AdminEmail,
		"item_type":   string(event.ItemType),
		"item_id":     itemID,
		"reason":      event.Reason,
		"changes":     changes,
		"timestamp":   event.Timestamp,
	})

	s.log.Info("audit event logged",
		zap.String("action", event.Action),
		zap.String("admin_email", event.AdminEmail),
		zap.String("item_type", string(event.ItemType)),
		zap.String("item_id", itemID),
		zap.Int("changes_count", len(changes)),
	)

	changeHistoryParams := changehistory.ChangeLog{
		UserID:     event.UserID,
		ProjectID:  event.ProjectID,
		BucketName: event.BucketName,
		AdminEmail: event.AdminEmail,
		ItemType:   event.ItemType,
		Reason:     event.Reason,
		Operation:  event.Action,
		Changes:    changes,
		Timestamp:  event.Timestamp,
	}
	if _, err := s.changeHistory.LogChange(ctx, changeHistoryParams); err != nil {
		s.log.Error("failed to log change history in database", zap.Error(err))
	}
}

// Run starts the audit logger processing loop.
func (s *Logger) Run(ctx context.Context) error {
	defer s.worker.Wait()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev := <-s.changeEvents:
			s.worker.Go(ctx, func() {
				s.logChangeEvent(ctx, ev)
			})
		}
	}
}

// Close shuts down the audit logger.
func (s *Logger) Close() error {
	close(s.changeEvents)
	s.worker.Wait()
	return nil
}

// TestToggleAuditLogger enables or disables the audit logger for testing purposes.
func (s *Logger) TestToggleAuditLogger(enabled bool) {
	s.config.Enabled = enabled
}
