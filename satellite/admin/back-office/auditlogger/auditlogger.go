// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package auditlogger

import (
	"time"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/analytics"
)

// Config contains configuration for the audit logger.
type Config struct {
	Enabled bool `help:"enable audit logging for admin operations" default:"false"`
	Caps    CapsConfig
}

// ItemType represents the type of item being audited.
type ItemType string

const (
	// ItemTypeUser represents a user item.
	ItemTypeUser ItemType = "User"
)

// Event represents an audit event for an admin operation.
type Event struct {
	Action     string    // e.g., "update_user", "freeze_account"
	AdminEmail string    // Who performed the action
	ItemType   ItemType  // e.g., "User", "Project", "Bucket"
	ItemID     uuid.UUID // ID of the affected item
	Reason     string    // Why the change was made
	Before     any       // Previous values
	After      any       // New values
	Timestamp  time.Time
}

// Logger implements Logger using Segment analytics service.
type Logger struct {
	log       *zap.Logger
	analytics *analytics.Service
	config    Config
}

// New creates a new Logger.
func New(log *zap.Logger, analytics *analytics.Service, config Config) *Logger {
	return &Logger{
		log:       log,
		analytics: analytics,
		config:    config,
	}
}

// LogChangeEvent logs an admin change event to Segment.
func (s *Logger) LogChangeEvent(userID uuid.UUID, event Event) {
	if !s.config.Enabled {
		return
	}

	changes := BuildChangeSet(event.Before, event.After, s.config.Caps)

	s.analytics.TrackAdminAuditEvent(userID, map[string]any{
		"action":      event.Action,
		"admin_email": event.AdminEmail,
		"item_type":   string(event.ItemType),
		"item_id":     event.ItemID.String(),
		"reason":      event.Reason,
		"changes":     changes,
		"timestamp":   event.Timestamp,
	})

	s.log.Info("audit event logged",
		zap.String("action", event.Action),
		zap.String("admin_email", event.AdminEmail),
		zap.String("item_type", string(event.ItemType)),
		zap.String("item_id", event.ItemID.String()),
		zap.Int("changes_count", len(changes)),
	)
}
