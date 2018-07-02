// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage"
)

// Store for streams
type Store interface {
	Meta(ctx context.Context, path paths.Path) (storage.Meta, error)
	Get(ctx context.Context, path paths.Path) (ranger.RangeCloser,
		storage.Meta, error)
	Put(ctx context.Context, path paths.Path, data io.Reader, metadata []byte,
		expiration time.Time) (storage.Meta, error)
	Delete(ctx context.Context, path paths.Path) error
	List(ctx context.Context, root, startAfter, endBefore paths.Path,
		recursive bool, limit int) (result []paths.Path, more bool, err error)
}
