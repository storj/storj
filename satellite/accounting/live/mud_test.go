// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package live

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
)

func TestOpenModuleCache(t *testing.T) {
	ctx := testcontext.New(t)
	log := zaptest.NewLogger(t)

	t.Run("unreachable redis", func(t *testing.T) {
		// like the classic peers, the satellite starts without redis, the cache reconnects on each operation
		cache, err := openModuleCache(ctx, log, Config{StorageBackend: "redis://127.0.0.1:1?db=0"})
		require.NoError(t, err)
		require.NotNil(t, cache)
		defer ctx.Check(cache.Close)
	})

	t.Run("invalid backend", func(t *testing.T) {
		_, err := openModuleCache(ctx, log, Config{StorageBackend: "unknown:"})
		require.Error(t, err)
	})

	t.Run("invalid redis url", func(t *testing.T) {
		_, err := openModuleCache(ctx, log, Config{StorageBackend: "redis://127.0.0.1:1?db=notanumber"})
		require.Error(t, err)
	})
}
