// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package postgreskv

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/internal/dbutil"
	"storj.io/storj/storage"
	"storj.io/storj/storage/postgreskv/schema"
)

const (
	defaultBatchSize = 10000
	defaultBucket    = ""
)

var (
	mon = monkit.Package()
)

// Client is the entrypoint into a postgreskv data store
type Client struct {
	URL    string
	pgConn *sql.DB
}

// New instantiates a new postgreskv client given db URL
func New(dbURL string) (*Client, error) {
	pgConn, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}

	dbutil.Configure(pgConn, mon)

	err = schema.PrepareDB(pgConn, dbURL)
	if err != nil {
		return nil, err
	}
	return &Client{
		URL:    dbURL,
		pgConn: pgConn,
	}, nil
}

// Put sets the value for the provided key.
func (client *Client) Put(ctx context.Context, key storage.Key, value storage.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	return client.PutPath(ctx, storage.Key(defaultBucket), key, value)
}

// PutPath sets the value for the provided key (in the given bucket).
func (client *Client) PutPath(ctx context.Context, bucket, key storage.Key, value storage.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return storage.ErrEmptyKey.New("")
	}
	q := `
		INSERT INTO pathdata (bucket, fullpath, metadata)
			VALUES ($1::BYTEA, $2::BYTEA, $3::BYTEA)
			ON CONFLICT (bucket, fullpath) DO UPDATE SET metadata = EXCLUDED.metadata
	`
	_, err = client.pgConn.Exec(q, []byte(bucket), []byte(key), []byte(value))
	return err
}

// Get looks up the provided key and returns its value (or an error).
func (client *Client) Get(ctx context.Context, key storage.Key) (_ storage.Value, err error) {
	defer mon.Task()(&ctx)(&err)
	return client.GetPath(ctx, storage.Key(defaultBucket), key)
}

// GetPath looks up the provided key (in the given bucket) and returns its value (or an error).
func (client *Client) GetPath(ctx context.Context, bucket, key storage.Key) (_ storage.Value, err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return nil, storage.ErrEmptyKey.New("")
	}

	q := "SELECT metadata FROM pathdata WHERE bucket = $1::BYTEA AND fullpath = $2::BYTEA"
	row := client.pgConn.QueryRow(q, []byte(bucket), []byte(key))

	var val []byte
	err = row.Scan(&val)
	if err == sql.ErrNoRows {
		return nil, storage.ErrKeyNotFound.New(key.String())
	}

	return val, Error.Wrap(err)
}

// Delete deletes the given key and its associated value.
func (client *Client) Delete(ctx context.Context, key storage.Key) (err error) {
	defer mon.Task()(&ctx)(&err)
	return client.DeletePath(ctx, storage.Key(defaultBucket), key)
}

// DeletePath deletes the given key (in the given bucket) and its associated value.
func (client *Client) DeletePath(ctx context.Context, bucket, key storage.Key) (err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return storage.ErrEmptyKey.New("")
	}

	q := "DELETE FROM pathdata WHERE bucket = $1::BYTEA AND fullpath = $2::BYTEA"
	result, err := client.pgConn.Exec(q, []byte(bucket), []byte(key))
	if err != nil {
		return err
	}
	numRows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if numRows == 0 {
		return storage.ErrKeyNotFound.New(key.String())
	}
	return nil
}

// List returns either a list of known keys, in order, or an error.
func (client *Client) List(ctx context.Context, first storage.Key, limit int) (_ storage.Keys, err error) {
	defer mon.Task()(&ctx)(&err)
	return storage.ListKeys(ctx, client, first, limit)
}

// Close closes the client
func (client *Client) Close() error {
	return client.pgConn.Close()
}

// GetAll finds all values for the provided keys (up to storage.LookupLimit).
// If more keys are provided than the maximum, an error will be returned.
func (client *Client) GetAll(ctx context.Context, keys storage.Keys) (_ storage.Values, err error) {
	defer mon.Task()(&ctx)(&err)
	return client.GetAllPath(ctx, storage.Key(defaultBucket), keys)
}

// GetAllPath finds all values for the provided keys (up to storage.LookupLimit)
// in the given bucket. if more keys are provided than the maximum, an error
// will be returned.
func (client *Client) GetAllPath(ctx context.Context, bucket storage.Key, keys storage.Keys) (_ storage.Values, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(keys) > storage.LookupLimit {
		return nil, storage.ErrLimitExceeded
	}

	q := `
		SELECT metadata
		FROM pathdata pd
			RIGHT JOIN
				unnest($2::BYTEA[]) WITH ORDINALITY pk(request, ord)
			ON (pd.fullpath = pk.request AND pd.bucket = $1::BYTEA)
		ORDER BY pk.ord
	`
	rows, err := client.pgConn.Query(q, []byte(bucket), pq.ByteaArray(keys.ByteSlices()))
	if err != nil {
		return nil, errs.Wrap(err)
	}
	values := make([]storage.Value, 0, len(keys))
	for rows.Next() {
		var value []byte
		if err := rows.Scan(&value); err != nil {
			return nil, errs.Wrap(errs.Combine(err, rows.Close()))
		}
		values = append(values, storage.Value(value))
	}
	return values, errs.Combine(rows.Err(), rows.Close())
}

type orderedPostgresIterator struct {
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

// Next fills in info for the next item in an ongoing listing.
func (opi *orderedPostgresIterator) Next(ctx context.Context, item *storage.ListItem) bool {
	defer mon.Task()(&ctx)(nil)
	if !opi.curRows.Next() {
		if err := opi.curRows.Close(); err != nil {
			opi.errEncountered = errs.Wrap(err)
			return false
		}
		if opi.curIndex < opi.batchSize {
			return false
		}
		if err := opi.curRows.Err(); err != nil {
			opi.errEncountered = errs.Wrap(err)
			return false
		}
		newRows, err := opi.nextQuery(ctx)
		if err != nil {
			opi.errEncountered = errs.Wrap(err)
			return false
		}
		opi.curRows = newRows
		opi.curIndex = 0
		if !opi.curRows.Next() {
			if err := opi.curRows.Close(); err != nil {
				opi.errEncountered = errs.Wrap(err)
			}
			return false
		}
	}
	var k, v []byte
	err := opi.curRows.Scan(&k, &v)
	if err != nil {
		opi.errEncountered = errs.Combine(errs.Wrap(err), errs.Wrap(opi.curRows.Close()))
		return false
	}
	item.Key = storage.Key(k)
	item.Value = storage.Value(v)
	opi.curIndex++
	if opi.curIndex == 1 && opi.lastKeySeen.Equal(item.Key) {
		return opi.Next(ctx, item)
	}
	if !opi.opts.Recurse && item.Key[len(item.Key)-1] == opi.delimiter && !item.Key.Equal(opi.opts.Prefix) {
		item.IsPrefix = true
		// i don't think this makes the most sense, but it's necessary to pass the storage testsuite
		item.Value = nil
	} else {
		item.IsPrefix = false
	}
	opi.lastKeySeen = item.Key
	return true
}

func (opi *orderedPostgresIterator) doNextQuery(ctx context.Context) (_ *sql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)
	start := opi.lastKeySeen
	if start == nil {
		start = opi.opts.First
	}
	var query string
	if !opi.opts.Recurse {
		if opi.opts.Reverse {
			query = "SELECT p, m FROM list_directory_reverse($1::BYTEA, $2::BYTEA, $3::BYTEA, $4) ld(p, m)"
		} else {
			query = "SELECT p, m FROM list_directory($1::BYTEA, $2::BYTEA, $3::BYTEA, $4) ld(p, m)"
		}
	} else {
		startCmp := ">="
		orderDir := ""
		if opi.opts.Reverse {
			startCmp = "<="
			orderDir = " DESC"
		}
		query = fmt.Sprintf(`
			SELECT fullpath, metadata
			  FROM pathdata
			 WHERE bucket = $1::BYTEA
			   AND ($2::BYTEA = ''::BYTEA OR fullpath >= $2::BYTEA)
			   AND ($2::BYTEA = ''::BYTEA OR fullpath < bytea_increment($2::BYTEA))
			   AND ($3::BYTEA = ''::BYTEA OR fullpath %s $3::BYTEA)
			 ORDER BY fullpath%s
			 LIMIT $4
		`, startCmp, orderDir)
	}
	return opi.client.pgConn.Query(query, []byte(opi.bucket), []byte(opi.opts.Prefix), []byte(start), opi.batchSize+1)
}

func (opi *orderedPostgresIterator) Close() error {
	return errs.Combine(opi.errEncountered, opi.curRows.Close())
}

func newOrderedPostgresIterator(ctx context.Context, pgClient *Client, opts storage.IterateOptions, batchSize int) (_ *orderedPostgresIterator, err error) {
	defer mon.Task()(&ctx)(&err)
	if opts.Prefix == nil {
		opts.Prefix = storage.Key("")
	}
	if opts.First == nil {
		opts.First = storage.Key("")
	}
	opi := &orderedPostgresIterator{
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

// Iterate iterates over items based on opts
func (client *Client) Iterate(ctx context.Context, opts storage.IterateOptions, fn func(context.Context, storage.Iterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	opi, err := newOrderedPostgresIterator(ctx, client, opts, defaultBatchSize)
	if err != nil {
		return err
	}
	defer func() {
		err = errs.Combine(err, opi.Close())
	}()

	return fn(ctx, opi)
}

// CompareAndSwap atomically compares and swaps oldValue with newValue
func (client *Client) CompareAndSwap(ctx context.Context, key storage.Key, oldValue, newValue storage.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	return client.CompareAndSwapPath(ctx, storage.Key(defaultBucket), key, oldValue, newValue)
}

// CompareAndSwapPath atomically compares and swaps oldValue with newValue in the given bucket
func (client *Client) CompareAndSwapPath(ctx context.Context, bucket, key storage.Key, oldValue, newValue storage.Value) (err error) {
	defer mon.Task()(&ctx)(&err)
	if key.IsZero() {
		return storage.ErrEmptyKey.New("")
	}

	if oldValue == nil && newValue == nil {
		q := "SELECT metadata FROM pathdata WHERE bucket = $1::BYTEA AND fullpath = $2::BYTEA"
		row := client.pgConn.QueryRow(q, []byte(bucket), []byte(key))
		var val []byte
		err = row.Scan(&val)
		if err == sql.ErrNoRows {
			return nil
		}
		if err != nil {
			return Error.Wrap(err)
		}
		return storage.ErrValueChanged.New(key.String())
	}

	if oldValue == nil {
		q := `
		INSERT INTO pathdata (bucket, fullpath, metadata) VALUES ($1::BYTEA, $2::BYTEA, $3::BYTEA)
			ON CONFLICT DO NOTHING
			RETURNING 1
		`
		row := client.pgConn.QueryRow(q, []byte(bucket), []byte(key), []byte(newValue))
		var val []byte
		err = row.Scan(&val)
		if err == sql.ErrNoRows {
			return storage.ErrValueChanged.New(key.String())
		}
		return Error.Wrap(err)
	}

	var row *sql.Row
	if newValue == nil {
		q := `
		WITH matching_key AS (
			SELECT * FROM pathdata WHERE bucket = $1::BYTEA AND fullpath = $2::BYTEA
		), updated AS (
			DELETE FROM pathdata
				USING matching_key mk
				WHERE pathdata.metadata = $3::BYTEA
					AND pathdata.bucket = mk.bucket
					AND pathdata.fullpath = mk.fullpath
				RETURNING 1
		)
		SELECT EXISTS(SELECT 1 FROM matching_key) AS key_present, EXISTS(SELECT 1 FROM updated) AS value_updated
		`
		row = client.pgConn.QueryRow(q, []byte(bucket), []byte(key), []byte(oldValue))
	} else {
		q := `
		WITH matching_key AS (
			SELECT * FROM pathdata WHERE bucket = $1::BYTEA AND fullpath = $2::BYTEA
		), updated AS (
			UPDATE pathdata
				SET metadata = $4::BYTEA
				FROM matching_key mk
				WHERE pathdata.metadata = $3::BYTEA
					AND pathdata.bucket = mk.bucket
					AND pathdata.fullpath = mk.fullpath
				RETURNING 1
		)
		SELECT EXISTS(SELECT 1 FROM matching_key) AS key_present, EXISTS(SELECT 1 FROM updated) AS value_updated;
		`
		row = client.pgConn.QueryRow(q, []byte(bucket), []byte(key), []byte(oldValue), []byte(newValue))
	}

	var keyPresent, valueUpdated bool
	err = row.Scan(&keyPresent, &valueUpdated)
	if err != nil {
		return Error.Wrap(err)
	}
	if !keyPresent {
		return storage.ErrKeyNotFound.New(key.String())
	}
	if !valueUpdated {
		return storage.ErrValueChanged.New(key.String())
	}
	return nil
}
