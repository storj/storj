// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package notifications

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/contact"
)

// RegisterCheckinCallback delivers the custom notifications of the satellites to the
// notification service, as they arrive with the check-in responses.
func RegisterCheckinCallback(log *zap.Logger, contactService *contact.Service, service *Service) {
	contactService.RegisterCheckinCallback(func(ctx context.Context, satelliteID storj.NodeID, resp *pb.CheckInResponse) error {
		// Stored notifications are never retracted, so a missing and an empty set are
		// handled the same way: there is nothing new to store.
		return ReceiveAll(ctx, log, service, satelliteID, resp.Notifications.GetNotifications())
	})
}

// ReceiveAll stores the custom notifications received from a satellite. Notifications
// without a key are ignored, as the key is required to identify them on re-delivery.
func ReceiveAll(ctx context.Context, log *zap.Logger, service *Service, satelliteID storj.NodeID, received []*pb.Notification) error {
	var group errs.Group
	for _, notification := range received {
		if notification.Key == "" {
			log.Warn("ignoring satellite notification without key", zap.Stringer("satellite_id", satelliteID))
			continue
		}

		// One unusable notification shouldn't hide the others.
		group.Add(service.ReceiveCustom(ctx, satelliteID, Custom{
			Key:       notification.Key,
			Title:     notification.Title,
			Message:   notification.Message,
			Link:      notification.Link,
			LinkLabel: notification.LinkLabel,
		}))
	}
	return group.Err()
}
