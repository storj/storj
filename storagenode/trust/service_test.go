// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/errs2"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
)

func TestGetSignee(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.New(t, 1, 1, 0)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	// make sure nodes are refreshed in db
	planet.Satellites[0].Discovery.Service.Refresh.TriggerWait()

	trust := planet.StorageNodes[0].Storage2.Trust

	canceledContext, cancel := context.WithCancel(ctx)
	cancel()

	var group errgroup.Group
	group.Go(func() error {
		_, err := trust.GetSignee(canceledContext, planet.Satellites[0].ID())
		if errs2.IsCanceled(err) {
			return nil
		}
		// if the other goroutine races us,
		// then we might get the certificate from the cache, however we shouldn't get an error
		return err
	})

	group.Go(func() error {
		cert, err := trust.GetSignee(ctx, planet.Satellites[0].ID())
		if err != nil {
			return err
		}
		if cert == nil {
			return errors.New("didn't get certificate")
		}
		return nil
	})

	assert.NoError(t, group.Wait())
}
