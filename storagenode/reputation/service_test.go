// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/assert"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/notifications"
)

func TestNewService(t *testing.T) {
	type args struct {
		log           *zap.Logger
		db            DB
		nodeID        storj.NodeID
		notifications *notifications.Service
	}
	var tests []struct {
		name string
		args func(t *testing.T) args

		want1 *Service
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := NewService(tArgs.log, tArgs.db, tArgs.nodeID, tArgs.notifications)

			assert.Equal(t, tt.want1, got1)
		})
	}
}

func TestService_Store(t *testing.T) {
	type args struct {
		ctx         context.Context
		stats       Stats
		satelliteID storj.NodeID
	}
	var tests []struct {
		name    string
		init    func(t *testing.T) *Service
		inspect func(r *Service, t *testing.T)

		args func(t *testing.T) args

		wantErr    bool
		inspectErr func(err error, t *testing.T)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			receiver := tt.init(t)
			err := receiver.Store(tArgs.ctx, tArgs.stats, tArgs.satelliteID)

			if tt.inspect != nil {
				tt.inspect(receiver, t)
			}

			if tt.wantErr {
				require.Error(t, err)
				if tt.inspectErr != nil {
					tt.inspectErr(err, t)
				}
			}
		})
	}
}

func Test_isSuspended(t *testing.T) {
	type args struct {
		new Stats
		old Stats
	}
	var tests []struct {
		name string
		args func(t *testing.T) args

		want1 bool
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := isSuspended(tArgs.new, tArgs.old)

			assert.Equal(t, tt.want1, got1)
		})
	}
}

func Test_newSuspensionNotification(t *testing.T) {
	type args struct {
		satelliteID storj.NodeID
		senderID    storj.NodeID
		time        time.Time
	}
	var tests []struct {
		name string
		args func(t *testing.T) args

		want1 notifications.NewNotification
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tArgs := tt.args(t)

			got1 := newSuspensionNotification(tArgs.satelliteID, tArgs.senderID, tArgs.time)

			assert.Equal(t, tt.want1, got1)
		})
	}
}
