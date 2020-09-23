// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package postgreskv

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/zeebo/errs"

	"storj.io/storj/private/tagsql"
	"storj.io/storj/storage"
)

type orderedPostgresIterator struct {
	client         *Client
	opts           *storage.IterateOptions
	delimiter      byte
	batchSize      int
	curIndex       int
	curRows        tagsql.Rows
	skipPrefix     bool
	lastKeySeen    storage.Key
	largestKey     storage.Key
	errEncountered error
}

func newOrderedPostgresIterator(ctx context.Context, cli *Client, opts storage.IterateOptions) (_ *orderedPostgresIterator, err error) {
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

	opi := &orderedPostgresIterator{
		client:    cli,
		opts:      &opts,
		delimiter: byte('/'),
		batchSize: opts.Limit,
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

func (opi *orderedPostgresIterator) Close() error {
	defer mon.Task()(nil)(nil)

	return errs.Combine(opi.curRows.Err(), opi.errEncountered, opi.curRows.Close())
}

// Next fills in info for the next item in an ongoing listing.
func (opi *orderedPostgresIterator) Next(ctx context.Context, item *storage.ListItem) bool {
	defer mon.Task()(&ctx)(nil)

	for {
		for {
			nextTask := mon.TaskNamed("check_next_row")(nil)
			next := opi.curRows.Next()
			nextTask(nil)
			if next {
				break
			}

			result := func() bool {
				defer mon.TaskNamed("acquire_new_query")(nil)(nil)

				if err := opi.curRows.Err(); err != nil && !errors.Is(err, sql.ErrNoRows) {
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
				return true
			}()
			if !result {
				return result
			}
		}

		var k, v []byte
		scanTask := mon.TaskNamed("scan_next_row")(nil)
		err := opi.curRows.Scan(&k, &v)
		scanTask(&err)
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

func (opi *orderedPostgresIterator) doNextQuery(ctx context.Context) (_ tagsql.Rows, err error) {
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
	var endLimitKey = opi.largestKey
	if endLimitKey == nil {
		// jackc/pgx will treat nil as a NULL value, while lib/pq treats it as
		// an empty string. We'll remove the ambiguity by making it a zero-length
		// byte slice instead
		endLimitKey = []byte{}
	}

	return opi.client.db.Query(ctx, fmt.Sprintf(`
		SELECT pd.fullpath, pd.metadata
		FROM pathdata pd
		WHERE pd.fullpath %s $1::BYTEA
			AND ($2::BYTEA = ''::BYTEA OR pd.fullpath < $2::BYTEA)
		ORDER BY pd.fullpath
		LIMIT $3
	`, gt), start, endLimitKey, opi.batchSize)
}
