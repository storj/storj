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
	"storj.io/storj/pkg/storage/segments"
)

type Passthrough struct {
	Segments segments.Store
}

func NewPassthrough(s segments.Store) *Passthrough {
	return &Passthrough{Segments: s}
}

var _ Store = (*Passthrough)(nil)

func (p *Passthrough) Meta(ctx context.Context, path paths.Path) (
	storage.Meta, error) {
	m, err := p.Segments.Meta(ctx, path)
	return convertMeta(m), err
}

func (p *Passthrough) Get(ctx context.Context, path paths.Path) (
	ranger.RangeCloser, storage.Meta, error) {
	rr, m, err := p.Segments.Get(ctx, path)
	return rr, convertMeta(m), err
}

func (p *Passthrough) Put(ctx context.Context, path paths.Path, data io.Reader,
	metadata []byte, expiration time.Time) (storage.Meta, error) {
	m, err := p.Segments.Put(ctx, path, data, metadata, expiration)
	return convertMeta(m), err
}

func (p *Passthrough) Delete(ctx context.Context, path paths.Path) error {
	return p.Segments.Delete(ctx, path)
}

func (p *Passthrough) List(ctx context.Context,
	prefix, startAfter, endBefore paths.Path, recursive bool, limit int,
	metaFlags uint64) (items []storage.ListItem, more bool, err error) {
	return p.Segments.List(ctx, prefix, startAfter, endBefore, recursive, limit,
		metaFlags)
}

func convertMeta(m segments.Meta) storage.Meta {
	// TODO
	return storage.Meta{}
}
