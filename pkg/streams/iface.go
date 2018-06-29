// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/ranger"
)

//Streams structure
type Streams struct {
	//segStore    segments.SegmentStore
	//streamStore streams.StreamStore
}

//Meta structure
type Meta struct {
	Modified   time.Time
	Expiration time.Time
	Data       []byte
}

//StreamStore interface
type StreamStore interface {
	Put(ctx context.Context, strmpath string, data io.Reader, metadata []byte,
		expiration time.Time) error
	Get(ctx context.Context, strmpath string) (ranger.Ranger, Meta, error)
	Delete(ctx context.Context, strmpath string) error
	List(ctx context.Context, startingPath, endingPath string) (
		strmpaths []string, truncated bool, err error)
}
