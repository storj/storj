// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cockroachkv

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"

	"github.com/zeebo/errs"

	"storj.io/storj/storage"
)

type orderedCockroachIterator struct {
	client         *Client
	opts           *storage.IterateOptions
	delimiter      byte
	batchSize      int
	curIndex       int
	curRows        *sql.Rows
	skipPrefix     bool
	lastKeySeen    storage.Key
	largestKey     storage.Key
	errEncountered error
}

func newOrderedCockroachIterator(ctx context.Context, cli *Client, opts storage.IterateOptions, batchSize int) (_ *orderedCockroachIterator, err error) {
	defer mon.Task()(&ctx)(&err)
	if opts.Prefix == nil {
		opts.Prefix = storage.Key("")
	}
	if opts.First == nil {
		opts.First = storage.Key("")
	}
	if opts.First.Less(opts.Prefix) {
		opts.First = opts.Prefix
	}

	opi := &orderedCockroachIterator{
		client:    cli,
		opts:      &opts,
		delimiter: byte('/'),
		batchSize: batchSize,
		curIndex:  0,
	}

	if len(opts.Prefix) > 0 {
		opi.largestKey = storage.AfterPrefix(opts.Prefix)
	}

	newRows, err := opi.doNextQuery(ctx)
	if err != nil {
		return nil, err
	}
	opi.curRows = newRows

	return opi, nil
}

func (opi *orderedCockroachIterator) Close() error {
	return errs.Combine(opi.errEncountered, opi.curRows.Close())
}

// Next fills in info for the next item in an ongoing listing.
func (opi *orderedCockroachIterator) Next(ctx context.Context, item *storage.ListItem) bool {
	defer mon.Task()(&ctx)(nil)

	for {
		for !opi.curRows.Next() {
			if err := opi.curRows.Err(); err != nil && err != sql.ErrNoRows {
				opi.errEncountered = errs.Wrap(err)
				return false
			}
			if err := opi.curRows.Close(); err != nil {
				opi.errEncountered = errs.Wrap(err)
				return false
			}
			if opi.curIndex < opi.batchSize {
				return false
			}
			newRows, err := opi.doNextQuery(ctx)
			if err != nil {
				opi.errEncountered = errs.Wrap(err)
				return false
			}
			opi.curRows = newRows
			opi.curIndex = 0
		}

		var k, v []byte
		err := opi.curRows.Scan(&k, &v)
		if err != nil {
			opi.errEncountered = errs.Wrap(err)
			return false
		}
		opi.curIndex++

		if !bytes.HasPrefix(k, []byte(opi.opts.Prefix)) {
			return false
		}

		item.Key = storage.Key(k)
		item.Value = storage.Value(v)
		item.IsPrefix = false

		if !opi.opts.Recurse {
			if idx := bytes.IndexByte(item.Key[len(opi.opts.Prefix):], opi.delimiter); idx >= 0 {
				item.Key = item.Key[:len(opi.opts.Prefix)+idx+1]
				item.Value = nil
				item.IsPrefix = true
			}
		}
		if opi.lastKeySeen.Equal(item.Key) {
			continue
		}

		opi.skipPrefix = item.IsPrefix
		opi.lastKeySeen = item.Key
		return true
	}
}

func (opi *orderedCockroachIterator) doNextQuery(ctx context.Context) (_ *sql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	gt := ">"
	start := opi.lastKeySeen

	if len(start) == 0 {
		start = opi.opts.First
		gt = ">="
	} else if opi.skipPrefix {
		start = storage.AfterPrefix(start)
		gt = ">="
	}

	return opi.client.db.Query(fmt.Sprintf(`
		SELECT pd.fullpath, pd.metadata
		FROM pathdata pd
		WHERE pd.fullpath %s $1:::BYTEA
			AND ($2:::BYTEA = '':::BYTEA OR pd.fullpath < $2:::BYTEA)
		LIMIT $3
	`, gt), start, []byte(opi.largestKey), opi.batchSize)
}
