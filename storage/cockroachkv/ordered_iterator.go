// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cockroachkv

import (
	"context"
	"database/sql"

	"storj.io/storj/storage"
)

type orderedCockroachIterator struct {
	client         *Client
	opts           *storage.IterateOptions
	bucket         storage.Key
	delimiter      byte
	batchSize      int
	curIndex       int
	curRows        *sql.Rows
	lastKeySeen    storage.Key
	errEncountered error
	nextQuery      func(context.Context) (*sql.Rows, error)
}

func newOrderedCockroachIterator(ctx context.Context, pgClient *Client, opts storage.IterateOptions, batchSize int) (_ *orderedCockroachIterator, err error) {
	defer mon.Task()(&ctx)(&err)
	if opts.Prefix == nil {
		opts.Prefix = storage.Key("")
	}
	if opts.First == nil {
		opts.First = storage.Key("")
	}
	opi := &orderedCockroachIterator{
		client:    pgClient,
		opts:      &opts,
		bucket:    storage.Key(defaultBucket),
		delimiter: byte('/'),
		batchSize: batchSize,
		curIndex:  0,
	}
	opi.nextQuery = opi.doNextQuery
	newRows, err := opi.nextQuery(ctx)
	if err != nil {
		return nil, err
	}
	opi.curRows = newRows
	return opi, nil
}

func (opi *orderedCockroachIterator) Close() error {
	//return errs.Combine(opi.errEncountered, opi.curRows.Close()) // TODO: Bring this back.
	return nil
}

// Next fills in info for the next item in an ongoing listing.
func (opi *orderedCockroachIterator) Next(ctx context.Context, item *storage.ListItem) bool {
	defer mon.Task()(&ctx)(nil)
	// TODO: Bring back the original implementation.
	return false
}

func (opi *orderedCockroachIterator) doNextQuery(ctx context.Context) (_ *sql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)
	// TODO: Implement iterating in a CockroachDB supported way.
	return nil, nil
}
