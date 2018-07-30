// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package streams

import (
	"context"
	"io"
	"time"

	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/segments"
)

// Passthrough implementation of stream store
type Passthrough struct {
	Segments segments.Store
}

// NewPassthrough stream store
func NewPassthrough(s segments.Store) *Passthrough {
	return &Passthrough{Segments: s}
}

var _ Store = (*Passthrough)(nil)

// Meta implements Store.Meta
func (p *Passthrough) Meta(ctx context.Context, path paths.Path) (Meta, error) {
	m, err := p.Segments.Meta(ctx, path)
	return convertMeta(m), err
}

// Get implements Store.Get
func (p *Passthrough) Get(ctx context.Context, path paths.Path) (
	ranger.RangeCloser, Meta, error) {
	rr, m, err := p.Segments.Get(ctx, path)
	return rr, convertMeta(m), err
}

// Put implements Store.Put
func (p *Passthrough) Put(ctx context.Context, path paths.Path, data io.Reader,
	metadata []byte, expiration time.Time) (Meta, error) {
	m, err := p.Segments.Put(ctx, path, data, metadata, expiration)
	return convertMeta(m), err
}

// Delete implements Store.Delete
func (p *Passthrough) Delete(ctx context.Context, path paths.Path) error {
	return p.Segments.Delete(ctx, path)
}

// List implements Store.List
func (p *Passthrough) List(ctx context.Context,
	prefix, startAfter, endBefore paths.Path, recursive bool, limit int,
	metaFlags uint32) (items []ListItem, more bool, err error) {
	segItems, more, err := p.Segments.List(ctx, prefix, startAfter, endBefore,
		recursive, limit, metaFlags)
	if err != nil {
		return nil, false, nil
	}

	items = make([]ListItem, len(segItems))
	for i, itm := range segItems {
		items[i] = ListItem{
			Path: itm.Path,
			Meta: convertMeta(itm.Meta),
		}
	}

	return items, more, nil
}

// convertMeta converts segment metadata to stream metadata
func convertMeta(m segments.Meta) Meta {
	return Meta{
		Modified:   m.Modified,
		Expiration: m.Expiration,
		Size:       m.Size,
		Data:       m.Data,
	}
}
