// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package piecedeletion_test

import (
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/satellite/metainfo/piecedeletion"
)

func TestLimitedJobs(t *testing.T) {
	{ // pop on an empty list
		q := piecedeletion.NewLimitedJobs(-1)
		list, ok := q.PopAll()
		require.False(t, ok)
		require.Nil(t, list)
	}

	{ // pop on a non-empty list
		q := piecedeletion.NewLimitedJobs(-1)

		job1 := randomJob(2)
		job2 := randomJob(3)

		// first push should always work
		require.True(t, q.TryPush(job1))
		// try push another, currently we don't have limits
		require.True(t, q.TryPush(job2))

		list, ok := q.PopAll()
		require.True(t, ok)
		require.Equal(t, []piecedeletion.Job{job1, job2}, list)

		// should be empty
		list, ok = q.PopAll()
		require.False(t, ok)
		require.Nil(t, list)

		// pushing another should fail
		require.False(t, q.TryPush(randomJob(1)))
	}
}

func TestLimitedJobs_Limiting(t *testing.T) {
	{
		q := piecedeletion.NewLimitedJobs(2)
		require.True(t, q.TryPush(randomJob(1)))
		require.True(t, q.TryPush(randomJob(1)))
		require.False(t, q.TryPush(randomJob(1)))
		require.False(t, q.TryPush(randomJob(1)))
	}

	{
		q := piecedeletion.NewLimitedJobs(2)
		require.True(t, q.TryPush(randomJob(1)))
		_, _ = q.PopAll()
		require.True(t, q.TryPush(randomJob(1)))
		_, _ = q.PopAll()
		require.False(t, q.TryPush(randomJob(1)))
		_, _ = q.PopAll()
		require.False(t, q.TryPush(randomJob(1)))
	}
}

func TestLimitedJobs_NoClose(t *testing.T) {
	{
		q := piecedeletion.NewLimitedJobs(2)
		job1, job2 := randomJob(1), randomJob(1)

		require.True(t, q.TryPush(job1))
		list := q.PopAllWithoutClose()
		require.Equal(t, []piecedeletion.Job{job1}, list)

		list = q.PopAllWithoutClose()
		require.Empty(t, list)

		require.True(t, q.TryPush(job2))
		list = q.PopAllWithoutClose()
		require.Equal(t, []piecedeletion.Job{job2}, list)
	}
}

func TestLimitedJobs_Concurrent(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	const N = 4

	q := piecedeletion.NewLimitedJobs(-1)

	jobs := []piecedeletion.Job{
		randomJob(1),
		randomJob(2),
		randomJob(3),
		randomJob(4),
	}

	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		i := i
		ctx.Go(func() error {
			defer wg.Done()
			if !q.TryPush(jobs[i]) {
				return errors.New("failed to add job")
			}
			return nil
		})
	}

	ctx.Go(func() error {
		wg.Wait()

		list, ok := q.PopAll()
		if !ok {
			return errors.New("failed to return jobs")
		}

		sort.Slice(list, func(i, k int) bool {
			return len(list[i].Pieces) < len(list[k].Pieces)
		})

		if !assert.Equal(t, jobs, list) {
			return errors.New("not equal")
		}

		return nil
	})
}

func TestLimitedJobs_NoRace(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	const N = 4

	q := piecedeletion.NewLimitedJobs(-1)

	jobs := []piecedeletion.Job{
		randomJob(1),
		randomJob(2),
		randomJob(3),
		randomJob(4),
	}
	for i := 0; i < N; i++ {
		i := i
		ctx.Go(func() error {
			_ = q.TryPush(jobs[i])
			return nil
		})
	}

	ctx.Go(func() error {
		_, _ = q.PopAll()
		return nil
	})
	ctx.Go(func() error {
		_ = q.PopAllWithoutClose()
		return nil
	})
}

func randomJob(n int) piecedeletion.Job {
	job := piecedeletion.Job{}
	for i := 0; i < n; i++ {
		job.Pieces = append(job.Pieces, testrand.PieceID())
	}
	return job
}
