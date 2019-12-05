// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cockroachkv

import (
	"bytes"
	"context"
	"database/sql"

	"github.com/cockroachdb/cockroach-go/crdb"
	"github.com/lib/pq"
	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/private/dbutil"
	"storj.io/storj/storage"
	"storj.io/storj/storage/cockroachkv/schema"
)

const defaultBatchSize = 128

var (
	mon = monkit.Package()
)

// Client is the entrypoint into a cockroachkv data store
type Client struct {
	db *sql.DB
}

// New instantiates a new postgreskv client given db URL
func New(dbURL string) (*Client, error) {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return nil, err
	}

	dbutil.Configure(db, mon)

	err = schema.PrepareDB(db)
	if err != nil {
		return nil, err
	}

	return NewWith(db), nil
}

// NewWith instantiates a new postgreskv client given db.
func NewWith(db *sql.DB) *Client {
	return &Client{db: db}
}

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
			VALUES ($1:::BYTEA, $2:::BYTEA)
			ON CONFLICT (fullpath) DO UPDATE SET metadata = EXCLUDED.metadata
	`

	_, err = client.db.Exec(q, []byte(key), []byte(value))
	return Error.Wrap(err)
}

// Get looks up the provided key and returns its value (or an error).
func (client *Client) Get(ctx context.Context, key storage.Key) (_ storage.Value, err error) {
	defer mon.Task()(&ctx)(&err)

	if key.IsZero() {
		return nil, storage.ErrEmptyKey.New("")
	}

	q := "SELECT metadata FROM pathdata WHERE fullpath = $1:::BYTEA"
	row := client.db.QueryRow(q, []byte(key))

	var val []byte
	err = row.Scan(&val)
	if err == sql.ErrNoRows {
		return nil, storage.ErrKeyNotFound.New("%q", key)
	}

	return val, Error.Wrap(err)
}

// GetAll finds all values for the provided keys (up to storage.LookupLimit).
// If more keys are provided than the maximum, an error will be returned.
func (client *Client) GetAll(ctx context.Context, keys storage.Keys) (_ storage.Values, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(keys) > storage.LookupLimit {
		return nil, storage.ErrLimitExceeded
	}

	q := `
		SELECT metadata
		FROM pathdata pd
			RIGHT JOIN
				unnest($1:::BYTEA[]) WITH ORDINALITY pk(request, ord)
			ON (pd.fullpath = pk.request)
		ORDER BY pk.ord
	`
	rows, err := client.db.Query(q, pq.ByteaArray(keys.ByteSlices()))
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

	q := "DELETE FROM pathdata WHERE fullpath = $1:::BYTEA"
	result, err := client.db.Exec(q, []byte(key))
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

// List returns either a list of known keys, in order, or an error.
func (client *Client) List(ctx context.Context, first storage.Key, limit int) (_ storage.Keys, err error) {
	defer mon.Task()(&ctx)(&err)
	return storage.ListKeys(ctx, client, first, limit)
}

// Iterate calls the callback with an iterator over the keys.
func (client *Client) Iterate(ctx context.Context, opts storage.IterateOptions, fn func(context.Context, storage.Iterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	opi, err := newOrderedCockroachIterator(ctx, client, opts, defaultBatchSize)
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
		q := "SELECT metadata FROM pathdata WHERE fullpath = $1:::BYTEA"
		row := client.db.QueryRow(q, []byte(key))

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
		INSERT INTO pathdata (fullpath, metadata) VALUES ($1:::BYTEA, $2:::BYTEA)
			ON CONFLICT DO NOTHING
			RETURNING 1
		`
		row := client.db.QueryRow(q, []byte(key), []byte(newValue))

		var val []byte
		err = row.Scan(&val)
		if err == sql.ErrNoRows {
			return storage.ErrValueChanged.New("%q", key)
		}
		return Error.Wrap(err)
	}

	return crdb.ExecuteTx(ctx, client.db, nil, func(txn *sql.Tx) error {
		q := "SELECT metadata FROM pathdata WHERE fullpath = $1:::BYTEA;"
		row := txn.QueryRowContext(ctx, q, []byte(key))

		var metadata []byte
		err = row.Scan(&metadata)
		if err == sql.ErrNoRows {
			// Row not found for this fullpath.
			// Potentially because another concurrent transaction changed the row.
			return storage.ErrKeyNotFound.New("%q", key)
		}
		if err != nil {
			return Error.Wrap(err)
		}

		if equal := bytes.Compare(metadata, oldValue); equal != 0 {
			// If the row is found but the metadata has been already changed
			// we can't continue to delete it.
			return storage.ErrValueChanged.New("%q", key)
		}

		var res sql.Result
		if newValue == nil {
			q = `
		DELETE FROM pathdata
			WHERE pathdata.fullpath = $1:::BYTEA
				AND pathdata.metadata = $2:::BYTEA
		`

			res, err = txn.ExecContext(ctx, q, []byte(key), []byte(oldValue))
		} else {
			q = `
		UPDATE pathdata
			SET metadata = $3:::BYTEA
			WHERE pathdata.fullpath = $1:::BYTEA
				AND pathdata.metadata = $2:::BYTEA
		`
			res, err = txn.ExecContext(ctx, q, []byte(key), []byte(oldValue), []byte(newValue))
		}

		if err != nil {
			return Error.Wrap(err)
		}

		affected, err := res.RowsAffected()
		if err != nil {
			return Error.Wrap(err)
		}

		if affected != 1 {
			return storage.ErrValueChanged.New("%q", key)
		}

		return nil
	})
}
