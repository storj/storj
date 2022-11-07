// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents

import (
	"context"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

// MockNotifier implements the Notifier interface.
type MockNotifier struct {
	log *zap.Logger
}

// NewMockNotifier is a constructor for MockNotifier.
func NewMockNotifier(log *zap.Logger) *MockNotifier {
	return &MockNotifier{
		log: log,
	}
}

// Notify stores the events in the Notifications field so they can be checked.
func (m *MockNotifier) Notify(ctx context.Context, satellite string, events []NodeEvent) (err error) {
	var nodeIDs string
	if len(events) == 0 {
		return nil
	}
	for _, e := range events {
		nodeIDs = nodeIDs + e.NodeID.String() + ","
	}
	nodeIDs = strings.TrimSuffix(nodeIDs, ",")

	eventString, err := typeToString(events[0].Event)
	if err != nil {
		return err
	}
	m.log.Info("node operator notified", zap.String("email", events[0].Email), zap.String("event", eventString), zap.String("node IDs", nodeIDs))
	return nil
}

func typeToString(event Type) (desc string, err error) {
	switch event {
	case Online:
		desc = "online"
	case Offline:
		desc = "offline"
	case Disqualified:
		desc = "disqualified"
	case UnknownAuditSuspended:
		desc = "unknown audit suspended"
	case UnknownAuditUnsuspended:
		desc = "unknown audit unsuspended"
	case OfflineSuspended:
		desc = "offline suspended"
	case OfflineUnsuspended:
		desc = "offline unsuspended"
	case BelowMinVersion:
		desc = "below minimum version"
	default:
		err = errs.New("event type has no description")
	}
	return desc, err
}
