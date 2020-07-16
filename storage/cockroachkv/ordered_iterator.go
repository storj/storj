// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cockroachkv

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/zeebo/errs"

	"storj.io/storj/private/dbutil/cockroachutil"
	"storj.io/storj/private/tagsql"
	"storj.io/storj/storage"
)

type orderedCockroachIterator struct {
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

func newOrderedCockroachIterator(ctx context.Context, cli *Client, opts storage.IterateOptions) (_ *orderedCockroachIterator, err error) {
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

	oci := &orderedCockroachIterator{
		client:    cli,
		opts:      &opts,
		delimiter: byte('/'),
		batchSize: opts.Limit,
		curIndex:  0,
	}

	if len(opts.Prefix) > 0 {
		oci.largestKey = storage.AfterPrefix(opts.Prefix)
	}

	newRows, err := oci.doNextQuery(ctx)
	if err != nil {
		return nil, err
	}
	oci.curRows = newRows

	return oci, nil
}

func (oci *orderedCockroachIterator) Close() error {
	defer mon.Task()(nil)(nil)

	return errs.Combine(oci.curRows.Err(), oci.errEncountered, oci.curRows.Close())
}

// Next fills in info for the next item in an ongoing listing.
func (oci *orderedCockroachIterator) Next(ctx context.Context, item *storage.ListItem) bool {
	defer mon.Task()(&ctx)(nil)

	for {
		for {
			nextTask := mon.TaskNamed("check_next_row")(nil)
			next := oci.curRows.Next()
			nextTask(nil)
			if next {
				break
			}

			result := func() bool {
				defer mon.TaskNamed("acquire_new_query")(nil)(nil)

				retry := false
				if err := oci.curRows.Err(); err != nil && !errors.Is(err, sql.ErrNoRows) {
					// This NeedsRetry needs to be exported here because it is
					// expected behavior for cockroach to return retryable errors
					// that will be captured in this Rows object.
					if cockroachutil.NeedsRetry(err) {
						mon.Event("needed_retry")
						retry = true
					} else {
						oci.errEncountered = errs.Wrap(err)
						return false
					}
				}
				if err := oci.curRows.Close(); err != nil {
					if cockroachutil.NeedsRetry(err) {
						mon.Event("needed_retry")
						retry = true
					} else {
						oci.errEncountered = errs.Wrap(err)
						return false
					}
				}
				if oci.curIndex < oci.batchSize && !retry {
					return false
				}
				newRows, err := oci.doNextQuery(ctx)
				if err != nil {
					oci.errEncountered = errs.Wrap(err)
					return false
				}
				oci.curRows = newRows
				oci.curIndex = 0
				return true
			}()
			if !result {
				return result
			}
		}

		var k, v []byte
		scanTask := mon.TaskNamed("scan_next_row")(nil)
		err := oci.curRows.Scan(&k, &v)
		scanTask(&err)
		if err != nil {
			oci.errEncountered = errs.Wrap(err)
			return false
		}
		oci.curIndex++

		if !bytes.HasPrefix(k, []byte(oci.opts.Prefix)) {
			return false
		}

		item.Key = storage.Key(k)
		item.Value = storage.Value(v)
		item.IsPrefix = false

		if !oci.opts.Recurse {
			if idx := bytes.IndexByte(item.Key[len(oci.opts.Prefix):], oci.delimiter); idx >= 0 {
				item.Key = item.Key[:len(oci.opts.Prefix)+idx+1]
				item.Value = nil
				item.IsPrefix = true
			}
		}
		if oci.lastKeySeen.Equal(item.Key) {
			continue
		}

		oci.skipPrefix = item.IsPrefix
		oci.lastKeySeen = item.Key
		return true
	}
}

func (oci *orderedCockroachIterator) doNextQuery(ctx context.Context) (_ tagsql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)

	gt := ">"
	start := oci.lastKeySeen

	largestKey := []byte(oci.largestKey)
	if largestKey == nil {
		// github.com/lib/pq would treat nil as an empty bytea array, while
		// github.com/jackc/pgx will treat nil as NULL. Make an explicit empty
		// byte array so that they'll work the same.
		largestKey = []byte{}
	}
	if len(start) == 0 {
		start = oci.opts.First
		gt = ">="
	} else if oci.skipPrefix {
		start = storage.AfterPrefix(start)
		gt = ">="
	}

	return oci.client.db.QueryContext(ctx, fmt.Sprintf(`
		SELECT pd.fullpath, pd.metadata
		FROM pathdata pd
		WHERE pd.fullpath %s $1:::BYTEA
			AND ($2:::BYTEA = '':::BYTEA OR pd.fullpath < $2:::BYTEA)
		LIMIT $3
	`, gt), start, largestKey, oci.batchSize)
}
