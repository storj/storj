// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"bytes"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
)

type writeThrottle struct {
	entered chan struct{}
	release chan error
}

type throttledWriter struct {
	writex int64
	write  []writeThrottle
	data   bytes.Buffer
}

func newThrottledWriter(maxWrites int) *throttledWriter {
	tw := &throttledWriter{
		writex: 0,
		write:  make([]writeThrottle, maxWrites),
	}
	for i := range tw.write {
		tw.write[i] = writeThrottle{
			entered: make(chan struct{}),
			release: make(chan error, 1),
		}
	}
	return tw
}

func (tw *throttledWriter) Write(data []byte) (n int, _ error) {
	index := atomic.AddInt64(&tw.writex, 1) - 1

	close(tw.write[index].entered)
	forceErr := <-tw.write[index].release

	n, writeErr := tw.data.Write(data)
	if writeErr != nil {
		return n, writeErr
	}

	return n, forceErr
}

func TestStdMultiWriteAbort(t *testing.T) {
	ctx := testcontext.New(t)

	stdout := newThrottledWriter(2)
	multi := newStdMultiWriteHandle(stdout)

	head := testrand.Bytes(256)
	tail := testrand.Bytes(256)

	part1, err := multi.NextPart(ctx, 256)
	require.NoError(t, err)
	ctx.Go(func() error {
		defer func() { _ = part1.Abort() }()

		_, err := part1.Write(head)
		if err == nil {
			return errors.New("expected an error")
		}
		return nil
	})

	part2, err := multi.NextPart(ctx, 256)
	require.NoError(t, err)
	ctx.Go(func() error {
		defer func() { _ = part2.Commit() }()

		// wait for the above part to enter write first
		<-stdout.write[0].entered
		_, err := part2.Write(tail)
		if err == nil {
			return errors.New("expected an error")
		}
		return nil
	})

	// wait until we enter both writes
	<-stdout.write[0].entered

	stdout.write[0].release <- errors.New("fail 0")
	stdout.write[1].release <- nil

	ctx.Wait()

	require.Equal(t, head, stdout.data.Bytes())
}
