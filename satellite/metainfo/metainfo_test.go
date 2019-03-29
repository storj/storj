// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo_test

import (
	"context"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite/console"
)

// mockAPIKeys is mock for api keys store of pointerdb
type mockAPIKeys struct {
	info console.APIKeyInfo
	err  error
}

// GetByKey return api key info for given key
func (keys *mockAPIKeys) GetByKey(ctx context.Context, key console.APIKey) (*console.APIKeyInfo, error) {
	return &keys.info, keys.err
}

func TestInvalidAPIKey(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 1, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	for _, invalidAPIKey := range []string{"", "invalid", "testKey"} {
		client, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], invalidAPIKey)
		require.NoError(t, err)

		_, _, err = client.CreateSegment(ctx, "hello", "world", 1, &pb.RedundancyScheme{}, 123, time.Now())
		assertUnauthenticated(t, err)

		_, err = client.CommitSegment(ctx, "testbucket", "testpath", 0, &pb.Pointer{}, nil)
		assertUnauthenticated(t, err)

		_, err = client.SegmentInfo(ctx, "testbucket", "testpath", 0)
		assertUnauthenticated(t, err)

		_, _, err = client.ReadSegment(ctx, "testbucket", "testpath", 0)
		assertUnauthenticated(t, err)

		_, err = client.DeleteSegment(ctx, "testbucket", "testpath", 0)
		assertUnauthenticated(t, err)

		_, _, err = client.ListSegments(ctx, "testbucket", "", "", "", true, 1, 0)
		assertUnauthenticated(t, err)
	}
}

func assertUnauthenticated(t *testing.T, err error) {
	t.Helper()

	if err, ok := status.FromError(errs.Unwrap(err)); ok {
		assert.Equal(t, codes.Unauthenticated, err.Code())
	} else {
		assert.Fail(t, "got unexpected error", "%T", err)
	}
}
