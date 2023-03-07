// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeevents

import (
	"context"
	"strings"

	"go.uber.org/zap"

	"storj.io/common/storj"
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
	seen := make(map[storj.NodeID]struct{})
	for _, e := range events {
		if _, ok := seen[e.NodeID]; !ok {
			seen[e.NodeID] = struct{}{}
			nodeIDs = nodeIDs + e.NodeID.String() + ","
		}
	}
	nodeIDs = strings.TrimSuffix(nodeIDs, ",")

	name, err := events[0].Event.Name()
	if err != nil {
		return err
	}
	m.log.Info("node operator notified", zap.String("email", events[0].Email), zap.String("event", name), zap.String("node IDs", nodeIDs))
	return nil
}
