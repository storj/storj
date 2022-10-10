// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
)

type mockAPIKeys struct {
	secret []byte
}

func (m *mockAPIKeys) GetByHead(ctx context.Context, head []byte) (*console.APIKeyInfo, error) {
	return &console.APIKeyInfo{Secret: m.secret}, nil
}

var _ APIKeys = (*mockAPIKeys)(nil)

func TestEndpoint_validateAuthN(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	secret, err := macaroon.NewSecret()
	require.NoError(t, err)

	key, err := macaroon.NewAPIKey(secret)
	require.NoError(t, err)

	keyNoLists, err := key.Restrict(macaroon.Caveat{DisallowLists: true})
	require.NoError(t, err)

	keyNoListsNoDeletes, err := keyNoLists.Restrict(macaroon.Caveat{DisallowDeletes: true})
	require.NoError(t, err)

	endpoint := Endpoint{
		log:     zaptest.NewLogger(t),
		apiKeys: &mockAPIKeys{secret: secret},
		top: endpointTop{
			Project:   func(name string) {},
			Partner:   func(name string) {},
			UserAgent: func(name string) {},
		},
	}

	now := time.Now()

	var canRead, canList, canDelete bool

	set1 := []verifyPermission{
		{
			action: macaroon.Action{
				Op:   macaroon.ActionDelete,
				Time: now,
			},
		},
		{
			action: macaroon.Action{
				Op:   macaroon.ActionRead,
				Time: now,
			},
			actionPermitted: &canRead,
			optional:        true,
		},
		{
			action: macaroon.Action{
				Op:   macaroon.ActionList,
				Time: now,
			},
			actionPermitted: &canList,
			optional:        true,
		},
	}
	set2 := []verifyPermission{
		{
			action: macaroon.Action{
				Op:   macaroon.ActionWrite,
				Time: now,
			},
		},
		{
			action: macaroon.Action{
				Op:   macaroon.ActionDelete,
				Time: now,
			},
			actionPermitted: &canDelete,
			optional:        true,
		},
	}
	set3 := []verifyPermission{
		{
			action: macaroon.Action{
				Op:   macaroon.ActionDelete,
				Time: now,
			},
		},
		{
			action: macaroon.Action{
				Op:   macaroon.ActionRead,
				Time: now,
			},
			actionPermitted: &canRead,
			optional:        true,
		},
		{
			action: macaroon.Action{
				Op:   macaroon.ActionList,
				Time: now,
			},
			actionPermitted: &canList,
			optional:        true,
		},
	}

	for i, tt := range [...]struct {
		key                                     *macaroon.APIKey
		permissions                             []verifyPermission
		wantCanRead, wantCanList, wantCanDelete bool
		wantErr                                 bool
	}{
		{
			key:     key,
			wantErr: true,
		},

		{
			key:         key,
			permissions: make([]verifyPermission, 2),
			wantErr:     true,
		},

		{
			key: key,
			permissions: []verifyPermission{
				{
					action: macaroon.Action{
						Op:   macaroon.ActionWrite,
						Time: now,
					},
					optional: true,
				},
				{
					action: macaroon.Action{
						Op:   macaroon.ActionDelete,
						Time: now,
					},
					optional: true,
				},
			},
			wantErr: true,
		},

		{
			key: key,
			permissions: []verifyPermission{
				{
					action: macaroon.Action{
						Op:   macaroon.ActionProjectInfo,
						Time: now,
					},
				},
			},
		},

		{
			key:         key,
			permissions: set1,
			wantCanRead: true,
			wantCanList: true,
		},
		{
			key:         keyNoLists,
			permissions: set1,
			wantCanRead: true,
		},
		{
			key:         keyNoListsNoDeletes,
			permissions: set1,
			wantErr:     true,
		},

		{
			key:           key,
			permissions:   set2,
			wantCanDelete: true,
		},
		{
			key:           keyNoLists,
			permissions:   set2,
			wantCanDelete: true,
		},
		{
			key:         keyNoListsNoDeletes,
			permissions: set2,
		},

		{
			key:         key,
			permissions: set3,
			wantCanRead: true,
			wantCanList: true,
		},
		{
			key:         keyNoLists,
			permissions: set3,
			wantCanRead: true,
		},
		{
			key:         keyNoListsNoDeletes,
			permissions: set3,
			wantErr:     true,
		},
	} {
		canRead, canList, canDelete = false, false, false // reset state

		rawKey := tt.key.SerializeRaw()
		ctxWithKey := consoleauth.WithAPIKey(ctx, rawKey)

		_, err := endpoint.validateAuthN(ctxWithKey, &pb.RequestHeader{ApiKey: rawKey}, tt.permissions...)

		assert.Equal(t, err != nil, tt.wantErr, i)
		assert.Equal(t, tt.wantCanRead, canRead, i)
		assert.Equal(t, tt.wantCanList, canList, i)
		assert.Equal(t, tt.wantCanDelete, canDelete, i)
	}
}
