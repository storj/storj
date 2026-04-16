// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package taskqueue

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/private/testredis"
)

func TestRunner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	redis, err := testredis.Mini(ctx)
	require.NoError(t, err)
	defer func() { require.NoError(t, redis.Close()) }()

	client, err := NewClient(ctx, Config{
		Address:  "redis://" + redis.Addr(),
		Group:    "test-runner",
		Consumer: "test-consumer",
	})
	require.NoError(t, err)
	defer func() { require.NoError(t, client.Close()) }()

	stream := "test-runner"

	// Push 20 items.
	for i := range 20 {
		err := client.Push(ctx, stream, testJob{
			NodeID: fmt.Sprintf("node-%d", i),
		})
		require.NoError(t, err)
	}

	var processed atomic.Int32

	runner := NewRunner[testJob](
		zaptest.NewLogger(t),
		RunnerConfig{
			WorkerCount: 2,
			Interval:    15 * time.Second,
			BatchSize:   5,
		},
		client,
		stream,
		ProcessorFunc[testJob](func(ctx context.Context, job testJob) {
			processed.Add(1)
		}),
	)

	var wg sync.WaitGroup
	defer wg.Wait()
	wg.Go(func() { _ = runner.Run(ctx) })

	// All 20 items must be processed well within 10 seconds.
	// Without the immediate-retry logic this would take 4 batches * 15s = 60s.
	require.Eventually(t, func() bool {
		return processed.Load() == 20
	}, 10*time.Second, 100*time.Millisecond)

	cancel()
}
