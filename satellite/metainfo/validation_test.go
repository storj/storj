// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

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
	"storj.io/storj/satellite/metainfo"
)

type mockAPIKeys struct {
	secret []byte
}

func (m *mockAPIKeys) GetByHead(ctx context.Context, head []byte) (*console.APIKeyInfo, error) {
	return &console.APIKeyInfo{Secret: m.secret}, nil
}

var _ metainfo.APIKeys = (*mockAPIKeys)(nil)

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

	endpoint := metainfo.TestingNewAPIKeysEndpoint(zaptest.NewLogger(t), &mockAPIKeys{secret: secret})

	now := time.Now()

	var canRead, canList, canDelete bool

	set1 := []metainfo.VerifyPermission{
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionDelete,
				Time: now,
			},
		},
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionRead,
				Time: now,
			},
			ActionPermitted: &canRead,
			Optional:        true,
		},
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionList,
				Time: now,
			},
			ActionPermitted: &canList,
			Optional:        true,
		},
	}
	set2 := []metainfo.VerifyPermission{
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionWrite,
				Time: now,
			},
		},
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionDelete,
				Time: now,
			},
			ActionPermitted: &canDelete,
			Optional:        true,
		},
	}
	set3 := []metainfo.VerifyPermission{
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionDelete,
				Time: now,
			},
		},
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionRead,
				Time: now,
			},
			ActionPermitted: &canRead,
			Optional:        true,
		},
		{
			Action: macaroon.Action{
				Op:   macaroon.ActionList,
				Time: now,
			},
			ActionPermitted: &canList,
			Optional:        true,
		},
	}

	for i, tt := range [...]struct {
		key                                     *macaroon.APIKey
		permissions                             []metainfo.VerifyPermission
		wantCanRead, wantCanList, wantCanDelete bool
		wantErr                                 bool
	}{
		{
			key:     key,
			wantErr: true,
		},

		{
			key:         key,
			permissions: make([]metainfo.VerifyPermission, 2),
			wantErr:     true,
		},

		{
			key: key,
			permissions: []metainfo.VerifyPermission{
				{
					Action: macaroon.Action{
						Op:   macaroon.ActionWrite,
						Time: now,
					},
					Optional: true,
				},
				{
					Action: macaroon.Action{
						Op:   macaroon.ActionDelete,
						Time: now,
					},
					Optional: true,
				},
			},
			wantErr: true,
		},

		{
			key: key,
			permissions: []metainfo.VerifyPermission{
				{
					Action: macaroon.Action{
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

		_, err := endpoint.ValidateAuthN(ctxWithKey, &pb.RequestHeader{ApiKey: rawKey}, tt.permissions...)

		assert.Equal(t, err != nil, tt.wantErr, i)
		assert.Equal(t, tt.wantCanRead, canRead, i)
		assert.Equal(t, tt.wantCanList, canList, i)
		assert.Equal(t, tt.wantCanDelete, canDelete, i)
	}
}
