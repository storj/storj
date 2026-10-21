// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/drpc/drpcctx"
	"storj.io/drpc/drpcmanager"
	"storj.io/drpc/drpcwire"
)

func TestIsCongested(t *testing.T) {
	const congestionThreshold = 0.8

	tests := []struct {
		name                  string
		maxConcurrentRequests int
		liveRequests          int32
		want                  bool
	}{
		// 0 means unlimited, so it must never report congestion — otherwise the
		// slow-upload check (skipped while congested) is silently disabled.
		{name: "unlimited is never congested", maxConcurrentRequests: 0, liveRequests: 1, want: false},
		{name: "unlimited stays uncongested under heavy load", maxConcurrentRequests: 0, liveRequests: 100000, want: false},
		{name: "negative is treated as unlimited", maxConcurrentRequests: -1, liveRequests: 100, want: false},

		// With a cap, congestion is strictly above the threshold (80% of 100).
		{name: "below threshold", maxConcurrentRequests: 100, liveRequests: 50, want: false},
		{name: "at threshold is not congested", maxConcurrentRequests: 100, liveRequests: 80, want: false},
		{name: "above threshold is congested", maxConcurrentRequests: 100, liveRequests: 81, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint := &Endpoint{
				config: Config{
					MaxConcurrentRequests:             tt.maxConcurrentRequests,
					MinUploadSpeedCongestionThreshold: congestionThreshold,
				},
			}
			endpoint.liveRequests.Store(tt.liveRequests)
			require.Equal(t, tt.want, endpoint.isCongested())
		})
	}
}

// TestWithTimeoutUnblocksStuckSend checks that withTimeout unblocks a send which
// is stuck writing to a peer that stopped reading (e.g. full TCP buffers).
func TestWithTimeoutUnblocksStuckSend(t *testing.T) {
	ctx := testcontext.New(t)

	// net.Pipe has no buffering: writes block until the other side reads.
	serverConn, clientConn := net.Pipe()
	defer ctx.Check(clientConn.Close)

	manager := drpcmanager.New(serverConn)
	defer ctx.Check(manager.Close)

	// The client sends an invoke, and then never reads anything.
	ctx.Go(func() error {
		wr := drpcwire.NewWriter(clientConn, 0)
		if err := wr.WritePacket(drpcwire.Packet{
			ID:   drpcwire.ID{Stream: 1, Message: 1},
			Kind: drpcwire.KindInvoke,
			Data: []byte("/test/Method"),
		}); err != nil {
			return err
		}
		return wr.Flush()
	})

	stream, _, err := manager.NewServerStream(ctx)
	require.NoError(t, err)

	cancelStream, ok := getCanceler(stream)
	require.True(t, ok)

	streamCtx := drpcctx.WithTransport(ctx, serverConn)

	done := make(chan error, 1)
	go func() {
		_, err := withTimeout(streamCtx, 100*time.Millisecond, cancelStream,
			func(ctx context.Context) (_ any, err error) {
				if err := stream.RawWrite(drpcwire.KindMessage, make([]byte, memory.MiB)); err != nil {
					return nil, err
				}
				return nil, stream.RawFlush()
			})
		done <- err
	}()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		// unblock the stuck goroutine so the test can finish
		_ = serverConn.Close()
		t.Fatal("withTimeout did not unblock the stuck send")
	}
}

func TestIsClientDisconnectError(t *testing.T) {
	managerClosed := errs.Class("manager closed")
	opErr := func(op string, err error) error {
		return &net.OpError{Op: op, Net: "tcp", Err: err}
	}

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "generic error", err: errors.New("disk is on fire"), want: false},
		{name: "broken pipe", err: opErr("write", os.NewSyscallError("write", syscall.EPIPE)), want: true},
		{name: "connection reset", err: opErr("read", os.NewSyscallError("read", syscall.ECONNRESET)), want: true},
		{name: "connection timed out", err: opErr("read", os.NewSyscallError("read", syscall.ETIMEDOUT)), want: true},
		{name: "closed connection", err: opErr("write", net.ErrClosed), want: true},
		{
			name: "wrapped connection timed out",
			err:  managerClosed.Wrap(opErr("read", os.NewSyscallError("read", syscall.ETIMEDOUT))),
			want: true,
		},
		{
			name: "connection timed out message only",
			err:  errors.New("manager closed: read tcp 1.2.3.4:4646->5.6.7.8:55586: read: connection timed out"),
			want: true,
		},
		{
			name: "closed connection message only",
			err:  errors.New("write tcp 1.2.3.4:4646->5.6.7.8:37648: use of closed network connection"),
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, isClientDisconnectError(tt.err))
		})
	}
}
