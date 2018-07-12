// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/paths"
)

type StreamStore interface {
	Put(ctx context.Context, path paths.Path, data io.Reader, metadata []byte, expiration time.Time) error
	// Get(ctx context.Context, path paths.Path) (ranger.Ranger, paths.Meta, error)
	// Delete(ctx context.Context, path paths.Path) error
	// List(ctx context.Context, startingPath, endingPath paths.Path) (paths []paths.Path, truncated bool, err error)
	// List​(ctx context.Context, root, startAfter, endBefore paths.Path, recursive ​bool​, limit ​int​) (result []paths.Path, more ​bool​, err ​error​)
}
