// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package postgreskv

import (
	"context"
	"database/sql"

	"github.com/lib/pq"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/storj/private/dbutil"
	"storj.io/storj/private/dbutil/pgutil"
	"storj.io/storj/private/tagsql"
	"storj.io/storj/storage"
	"storj.io/storj/storage/postgreskv/schema"
)

var (
	mon = monkit.Package()
)

// Client is the entrypoint into a postgreskv data store
type Client struct {
	db          tagsql.DB
	dbURL       string
	lookupLimit int
}

// New instantiates a new postgreskv client given db URL.
func New(dbURL string) (*Client, error) {
	dbURL = pgutil.CheckApplicationName(dbURL)

	db, err := tagsql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}

	dbutil.Configure(db, "postgreskv", mon)
	return NewWith(db, dbURL), nil
}

// NewWith instantiates a new postgreskv client given db.
func NewWith(db tagsql.DB, dbURL string) *Client {
	return &Client{db: db, lookupLimit: storage.DefaultLookupLimit, dbURL: dbURL}
}

// MigrateToLatest migrates to latest schema version.
func (client *Client) MigrateToLatest(ctx context.Context) error {
	return schema.PrepareDB(ctx, client.db, client.dbURL)
}

// SetLookupLimit sets the lookup limit.
func (client *Client) SetLookupLimit(v int) { client.lookupLimit = v }

// LookupLimit returns the maximum limit that is allowed.
func (client *Client) LookupLimit() int { return client.lookupLimit }

// Close closes the client
func (client *Client) Close() error {
	return client.db.Close()
}

// Put sets the value for the provided key.
func (client *Client) Put(ctx context.Context, key storage.Key, value storage.Value) (err error) {
	defer mon.Task()(&ctx)(&err)

	if key.IsZero() {
		return storage.ErrEmptyKey.New("")
	}

	q := `
		INSERT INTO pathdata (fullpath, metadata)
			VALUES ($1::BYTEA, $2::BYTEA)
			ON CONFLICT (fullpath) DO UPDATE SET metadata = EXCLUDED.metadata
	`

	_, err = client.db.Exec(ctx, q, []byte(key), []byte(value))
	return Error.Wrap(err)
}

// Get looks up the provided key and returns its value (or an error).
func (client *Client) Get(ctx context.Context, key storage.Key) (_ storage.Value, err error) {
	defer mon.Task()(&ctx)(&err)

	if key.IsZero() {
		return nil, storage.ErrEmptyKey.New("")
	}

	q := "SELECT metadata FROM pathdata WHERE fullpath = $1::BYTEA"
	row := client.db.QueryRow(ctx, q, []byte(key))

	var val []byte
	err = row.Scan(&val)
	if err == sql.ErrNoRows {
		return nil, storage.ErrKeyNotFound.New("%q", key)
	}

	return val, Error.Wrap(err)
}

// GetAll finds all values for the provided keys (up to LookupLimit).
// If more keys are provided than the maximum, an error will be returned.
func (client *Client) GetAll(ctx context.Context, keys storage.Keys) (_ storage.Values, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(keys) > client.lookupLimit {
		return nil, storage.ErrLimitExceeded
	}

	q := `
		SELECT metadata
		FROM pathdata pd
			RIGHT JOIN
				unnest($1::BYTEA[]) WITH ORDINALITY pk(request, ord)
			ON (pd.fullpath = pk.request)
		ORDER BY pk.ord
	`
	rows, err := client.db.Query(ctx, q, pq.ByteaArray(keys.ByteSlices()))
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() { err = errs.Combine(err, Error.Wrap(rows.Close())) }()

	values := make([]storage.Value, 0, len(keys))
	for rows.Next() {
		var value []byte
		if err := rows.Scan(&value); err != nil {
			return nil, Error.Wrap(err)
		}
		values = append(values, storage.Value(value))
	}

	return values, Error.Wrap(rows.Err())
}

// Delete deletes the given key and its associated value.
func (client *Client) Delete(ctx context.Context, key storage.Key) (err error) {
	defer mon.Task()(&ctx)(&err)

	if key.IsZero() {
		return storage.ErrEmptyKey.New("")
	}

	q := "DELETE FROM pathdata WHERE fullpath = $1::BYTEA"
	result, err := client.db.Exec(ctx, q, []byte(key))
	if err != nil {
		return err
	}

	numRows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if numRows == 0 {
		return storage.ErrKeyNotFound.New("%q", key)
	}
	return nil
}

// DeleteMultiple deletes keys ignoring missing keys
func (client *Client) DeleteMultiple(ctx context.Context, keys []storage.Key) (_ storage.Items, err error) {
	defer mon.Task()(&ctx, len(keys))(&err)

	rows, err := client.db.QueryContext(ctx, `
		DELETE FROM pathdata
		WHERE fullpath = any($1::BYTEA[])
		RETURNING fullpath, metadata`,
		pq.ByteaArray(storage.Keys(keys).ByteSlices()))
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, rows.Close())
	}()

	var items storage.Items
	for rows.Next() {
		var key, value []byte
		err := rows.Scan(&key, &value)
		if err != nil {
			return items, err
		}
		items = append(items, storage.ListItem{
			Key:   key,
			Value: value,
		})
	}

	return items, rows.Err()
}

// List returns either a list of known keys, in order, or an error.
func (client *Client) List(ctx context.Context, first storage.Key, limit int) (_ storage.Keys, err error) {
	defer mon.Task()(&ctx)(&err)
	return storage.ListKeys(ctx, client, first, limit)
}

// Iterate calls the callback with an iterator over the keys.
func (client *Client) Iterate(ctx context.Context, opts storage.IterateOptions, fn func(context.Context, storage.Iterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	if opts.Limit <= 0 || opts.Limit > client.lookupLimit {
		opts.Limit = client.lookupLimit
	}

	return client.IterateWithoutLookupLimit(ctx, opts, fn)
}

// IterateWithoutLookupLimit calls the callback with an iterator over the keys, but doesn't enforce default limit on opts.
func (client *Client) IterateWithoutLookupLimit(ctx context.Context, opts storage.IterateOptions, fn func(context.Context, storage.Iterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)

	opi, err := newOrderedPostgresIterator(ctx, client, opts)
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

	if key.IsZero() {
		return storage.ErrEmptyKey.New("")
	}

	if oldValue == nil && newValue == nil {
		q := "SELECT metadata FROM pathdata WHERE fullpath = $1::BYTEA"
		row := client.db.QueryRow(ctx, q, []byte(key))

		var val []byte
		err = row.Scan(&val)
		if err == sql.ErrNoRows {
			return nil
		}

		if err != nil {
			return Error.Wrap(err)
		}

		return storage.ErrValueChanged.New("%q", key)
	}

	if oldValue == nil {
		q := `
		INSERT INTO pathdata (fullpath, metadata) VALUES ($1::BYTEA, $2::BYTEA)
			ON CONFLICT DO NOTHING
			RETURNING 1
		`
		row := client.db.QueryRow(ctx, q, []byte(key), []byte(newValue))

		var val []byte
		err = row.Scan(&val)
		if err == sql.ErrNoRows {
			return storage.ErrValueChanged.New("%q", key)
		}

		return Error.Wrap(err)
	}

	var row *sql.Row
	if newValue == nil {
		q := `
		WITH matching_key AS (
			SELECT * FROM pathdata WHERE fullpath = $1::BYTEA
		), updated AS (
			DELETE FROM pathdata
				USING matching_key mk
				WHERE pathdata.metadata = $2::BYTEA
					AND pathdata.fullpath = mk.fullpath
				RETURNING 1
		)
		SELECT EXISTS(SELECT 1 FROM matching_key) AS key_present, EXISTS(SELECT 1 FROM updated) AS value_updated
		`
		row = client.db.QueryRow(ctx, q, []byte(key), []byte(oldValue))
	} else {
		q := `
		WITH matching_key AS (
			SELECT * FROM pathdata WHERE fullpath = $1::BYTEA
		), updated AS (
			UPDATE pathdata
				SET metadata = $3::BYTEA
				FROM matching_key mk
				WHERE pathdata.metadata = $2::BYTEA
					AND pathdata.fullpath = mk.fullpath
				RETURNING 1
		)
		SELECT EXISTS(SELECT 1 FROM matching_key) AS key_present, EXISTS(SELECT 1 FROM updated) AS value_updated;
		`
		row = client.db.QueryRow(ctx, q, []byte(key), []byte(oldValue), []byte(newValue))
	}

	var keyPresent, valueUpdated bool
	err = row.Scan(&keyPresent, &valueUpdated)
	if err != nil {
		return Error.Wrap(err)
	}

	if !keyPresent {
		return storage.ErrKeyNotFound.New("%q", key)
	}

	if !valueUpdated {
		return storage.ErrValueChanged.New("%q", key)
	}

	return nil
}
