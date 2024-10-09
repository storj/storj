// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/shared/nodetag"
)

func TestGetTags(t *testing.T) {
	ctx := testcontext.New(t)
	cfg := Config{
		Tags: SignedTags{},
		SelfSignedTags: []string{
			"foo=bar",
		},
	}
	id := testidentity.MustPregeneratedIdentity(0, storj.LatestIDVersion())
	tags, err := GetTags(ctx, cfg, id)
	require.NoError(t, err)
	require.Len(t, tags.Tags, 1)
	_, err = nodetag.Verify(ctx, tags.Tags[0], signing.SigneeFromPeerIdentity(id.PeerIdentity()))
	require.NoError(t, err)

}
