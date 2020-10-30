// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"context"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/notifications"
)

// Service is the reputation service.
//
// architecture: Service
type Service struct {
	log *zap.Logger

	db            DB
	nodeID        storj.NodeID
	notifications *notifications.Service
}

// NewService creates new instance of service.
func NewService(log *zap.Logger, db DB, nodeID storj.NodeID, notifications *notifications.Service) *Service {
	return &Service{
		log:           log,
		db:            db,
		nodeID:        nodeID,
		notifications: notifications,
	}
}

// Store stores reputation stats into db, and notify's in case of offline suspension.
func (s *Service) Store(ctx context.Context, stats Stats, satelliteID storj.NodeID) error {
	if err := s.db.Store(ctx, stats); err != nil {
		return err
	}

	if stats.DisqualifiedAt == nil && stats.OfflineSuspendedAt != nil {
		s.notifyOfflineSuspension(ctx, satelliteID)
	}

	return nil
}

// NotifyOfflineSuspension notifies storagenode about offline suspension.
func (s *Service) notifyOfflineSuspension(ctx context.Context, satelliteID storj.NodeID) {
	notification := NewSuspensionNotification(satelliteID, s.nodeID)

	_, err := s.notifications.Receive(ctx, notification)
	if err != nil {
		s.log.Sugar().Errorf("Failed to receive notification", err.Error())
	}
}

// NewSuspensionNotification - returns offline suspension notification.
func NewSuspensionNotification(satelliteID storj.NodeID, senderID storj.NodeID) (_ notifications.NewNotification) {
	return notifications.NewNotification{
		SenderID: senderID,
		Type:     notifications.TypeCustom,
		Title:    "Your Node was suspended!",
		Message:  "This is a reminder that your Storage Node on " + satelliteID.String() + "Satellite is suspended",
	}
}
