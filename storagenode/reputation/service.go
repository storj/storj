// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"context"
	"time"

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
	rep, err := s.db.Get(ctx, satelliteID)
	if err != nil {
		return err
	}

	err = s.db.Store(ctx, stats)
	if err != nil {
		return err
	}

	if stats.DisqualifiedAt == nil && isSuspended(stats, *rep) {
		notification := newSuspensionNotification(satelliteID, s.nodeID, *stats.OfflineSuspendedAt)

		_, err = s.notifications.Receive(ctx, notification)
		if err != nil {
			s.log.Sugar().Errorf("Failed to receive notification", err.Error())
		}
	}

	return nil
}

// isSuspended returns if there's new downtime suspension.
func isSuspended(new, old Stats) bool {
	if new.OfflineSuspendedAt == nil {
		return false
	}

	if old.OfflineSuspendedAt == nil {
		return true
	}

	if !old.OfflineSuspendedAt.Equal(*new.OfflineSuspendedAt) {
		return true
	}

	return false
}

// newSuspensionNotification - returns offline suspension notification.
func newSuspensionNotification(satelliteID storj.NodeID, senderID storj.NodeID, time time.Time) (_ notifications.NewNotification) {
	return notifications.NewNotification{
		SenderID: senderID,
		Type:     notifications.TypeSuspension,
		Title:    "Your Node is suspended since " + time.String(),
		Message:  "This is a reminder that your StorageNode is suspended on Satellite " + satelliteID.String(),
	}
}
