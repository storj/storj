// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/cfgstruct"
	"storj.io/common/testcontext"
	"storj.io/storj/private/web"
)

func TestNewIPRateLimiter(t *testing.T) {
	// create a rate limiter with defaults except NumLimits = 2
	config := web.RateLimiterConfig{}
	cfgstruct.Bind(&pflag.FlagSet{}, &config, cfgstruct.UseDevDefaults())
	config.NumLimits = 2
	rateLimiter := web.NewIPRateLimiter(config, zaptest.NewLogger(t))

	// run ratelimiter cleanup until end of test
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	ctx2, cancel := context.WithCancel(ctx)
	defer cancel()
	ctx.Go(func() error {
		rateLimiter.Run(ctx2)
		return nil
	})

	// make the default HTTP handler return StatusOK
	handler := rateLimiter.Limit(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// expect burst number of successes
	testWithAddress(ctx, t, "192.168.1.1:5000", rateLimiter.Burst(), handler)
	// expect similar results for a different IP
	testWithAddress(ctx, t, "127.0.0.1:5000", rateLimiter.Burst(), handler)
	// expect similar results for a different IP
	testWithAddress(ctx, t, "127.0.0.100:5000", rateLimiter.Burst(), handler)
	// expect original IP to work again because numLimits == 2
	testWithAddress(ctx, t, "192.168.1.1:5000", rateLimiter.Burst(), handler)
}

func testWithAddress(ctx context.Context, t *testing.T, remoteAddress string, burst int, handler http.Handler) {
	// create HTTP request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "", nil)
	require.NoError(t, err)
	req.RemoteAddr = remoteAddress

	// expect burst number of successes
	for x := 0; x < burst; x++ {
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		assert.Equal(t, rr.Code, http.StatusOK, remoteAddress)
	}

	// then expect failure
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert.Equal(t, rr.Code, http.StatusTooManyRequests, remoteAddress)
}
