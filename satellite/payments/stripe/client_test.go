// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe_test

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/form"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/time2"
	stripe1 "storj.io/storj/satellite/payments/stripe"
)

var backendError = &stripe.Error{
	APIResource: stripe.APIResource{
		LastResponse: &stripe.APIResponse{
			Header: http.Header{
				"Stripe-Should-Retry": []string{"true"},
			},
			StatusCode: http.StatusTooManyRequests,
		},
	},
}

type mockBackend struct {
	calls int64
}

func (b *mockBackend) Call(method, path, key string, params stripe.ParamsContainer, v stripe.LastResponseSetter) error {
	b.calls++
	return backendError
}

func (b *mockBackend) CallStreaming(method, path, key string, params stripe.ParamsContainer, v stripe.StreamingLastResponseSetter) error {
	return b.Call("", "", "", nil, nil)
}

func (b *mockBackend) CallRaw(method, path, key string, body *form.Values, params *stripe.Params, v stripe.LastResponseSetter) error {
	return b.Call("", "", "", nil, nil)
}

func (b *mockBackend) CallMultipart(method, path, key, boundary string, body *bytes.Buffer, params *stripe.Params, v stripe.LastResponseSetter) error {
	return b.Call("", "", "", nil, nil)
}

func (b *mockBackend) SetMaxNetworkRetries(max int64) {}

func TestBackendWrapper(t *testing.T) {
	tm := time2.NewMachine()
	retryCfg := stripe1.RetryConfig{
		InitialBackoff: time.Millisecond,
		MaxBackoff:     3 * time.Millisecond,
		Multiplier:     2,
		MaxRetries:     5,
	}
	backend := stripe1.NewBackendWrapper(zaptest.NewLogger(t), stripe.APIBackend, retryCfg)
	mock := &mockBackend{}
	backend.TestSwapBackend(mock)
	backend.TestSwapClock(tm.Clock())

	newCall := func(t *testing.T, ctx context.Context) (wait func(context.Context) error) {
		mock.calls = 0
		done := make(chan error)

		go func() {
			done <- backend.Call("", "", "", &stripe.Params{Context: ctx}, nil)
		}()

		wait = func(ctx context.Context) error {
			select {
			case err := <-done:
				return err
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return wait
	}

	t.Run("backoff intervals", func(t *testing.T) {
		ctx := testcontext.New(t)
		wait := newCall(t, ctx)

		expectedBackoff := retryCfg.InitialBackoff
		for i := 0; i < int(retryCfg.MaxRetries); i++ {
			if !tm.BlockThenAdvance(ctx, 1, expectedBackoff) {
				t.Fatal("failed waiting for the client to attempt retry", i+1)
			}
			expectedBackoff *= 2
			if expectedBackoff > retryCfg.MaxBackoff {
				expectedBackoff = retryCfg.MaxBackoff
			}
		}

		require.Error(t, wait(ctx), backendError)
		require.Equal(t, retryCfg.MaxRetries+1, mock.calls)
	})

	t.Run("context cancellation", func(t *testing.T) {
		t.Run("before retries", func(t *testing.T) {
			ctx := testcontext.New(t)
			subCtx, cancel := context.WithCancel(ctx)
			cancel()

			wait := newCall(t, subCtx)
			require.ErrorIs(t, wait(ctx), context.Canceled)
			require.Zero(t, mock.calls)
		})

		t.Run("during retries", func(t *testing.T) {
			ctx := testcontext.New(t)
			subCtx, cancel := context.WithCancel(ctx)
			wait := newCall(t, subCtx)

			if !tm.BlockThenAdvance(ctx, 1, retryCfg.InitialBackoff) {
				t.Fatal("failed waiting for the client to attempt first retry")
			}

			cancel()
			require.ErrorIs(t, wait(ctx), context.Canceled)
			require.Equal(t, int64(2), mock.calls)
		})
	})
}
