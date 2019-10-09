// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package sqlitekv

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/storage"
)

var (
	// Error is the default sqlitekv errs class
	Error = errs.Class("sqlitekv error")
	mon   = monkit.Package()
)

const (
	defaultBatchSize = 10000
	defaultBucket    = ""
)

// Client is the entrypoint into a sqlitekv data store
type Client struct {
	URL        string
	sqliteConn *sql.DB
}

// New instantiates a new sqlitekv client given db URL
func New(dbURL string) (*Client, error) {
	sqliteConn, err := sql.Open("sqlite3", dbURL)
	if err != nil {
		return nil, err
	}

	err = prepareDB(sqliteConn)
	if err != nil {
		return nil, err
	}
	fmt.Println("prepared")

	return &Client{
		URL:        dbURL,
		sqliteConn: sqliteConn,
	}, nil
}

func prepareDB(db *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS pathdata (
			bucket BLOB NOT NULL,
			fullpath BLOB NOT NULL,
			metadata BLOB NOT NULL,
			PRIMARY KEY ( bucket, fullpath )
		);
	`
	db.Exec(query)
	return nil
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
	query := `
		INSERT INTO pathdata (bucket, fullpath, metadata)
		VALUES (?,?,?)
		ON CONFLICT (bucket, fullpath) DO UPDATE SET metadata = EXCLUDED.metadata
	`
	_, err = client.sqliteConn.Exec(query, []byte(bucket), []byte(key), []byte(value))
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

	query := "SELECT metadata FROM pathdata WHERE bucket = ? AND fullpath = ?"
	row := client.sqliteConn.QueryRow(query, []byte(bucket), []byte(key))

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
	return client.GetAllPath(ctx, storage.Key(defaultBucket), keys)
}

// GetAllPath finds all values for the provided keys (up to storage.LookupLimit)
// in the given bucket. If more keys are provided than the maximum, an error
// will be returned.
func (client *Client) GetAllPath(ctx context.Context, bucket storage.Key, keys storage.Keys) (_ storage.Values, err error) {
	defer mon.Task()(&ctx)(&err)
	if len(keys) > storage.LookupLimit {
		return nil, storage.ErrLimitExceeded
	}
	values := make([]storage.Value, len(keys))

	for i, key := range keys.ByteSlices() {
		query := `
			SELECT metadata FROM pathdata
			WHERE bucket = ? AND fullpath = ?
			ORDER BY fullpath
		`
		row := client.sqliteConn.QueryRow(query, []byte(bucket), key)
		var value []byte
		if err := row.Scan(&value); err != nil {
			return nil, err
		}
		values[i] = storage.Value(value)
	}
	return values, err
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

	query := "DELETE FROM pathdata WHERE bucket = ? AND fullpath = ?"
	result, err := client.sqliteConn.Exec(query, []byte(bucket), []byte(key))
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

// Iterate iterates over items based on opts
func (client *Client) Iterate(ctx context.Context, opts storage.IterateOptions, fn func(context.Context, storage.Iterator) error) (err error) {
	defer mon.Task()(&ctx)(&err)
	opi, err := newOrderedSqliteIterator(ctx, client, opts, defaultBatchSize)
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
		query := "SELECT metadata FROM pathdata WHERE bucket = ? AND fullpath = ?"
		row := client.sqliteConn.QueryRow(query, []byte(bucket), []byte(key))
		var val []byte
		err = row.Scan(&val)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return Error.Wrap(err)
		}
		return storage.ErrValueChanged.New("%q", key)
	}

	if oldValue == nil {
		query := `
			INSERT INTO pathdata (bucket, fullpath, metadata)
			VALUES (?,?,?)
			ON CONFLICT DO NOTHING
		`
		result, err := client.sqliteConn.Exec(query, []byte(bucket), []byte(key), []byte(newValue))
		if err != nil {
			return Error.Wrap(err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return Error.Wrap(err)
		}
		if count < 1 {
			return storage.ErrValueChanged.New("%q", key)
		}
		return Error.Wrap(err)
	}

	var keyPresent, valueUpdated bool
	if newValue == nil {
		queryMatch := `SELECT * FROM pathdata WHERE bucket = ? AND fullpath = ?`
		result, err := client.sqliteConn.Exec(queryMatch, []byte(bucket), []byte(key))
		if err != nil {
			return Error.Wrap(err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return Error.Wrap(err)
		}
		if count > 0 {
			keyPresent = true
		}
		if keyPresent {
			query := `
				DELETE FROM pathdata 
				WHERE bucket = ? AND fullpath = ? and metadata = ?
			`
			result, err = client.sqliteConn.Exec(query, []byte(bucket), []byte(key), []byte(oldValue))
			if err != nil {
				return Error.Wrap(err)
			}
			count, err = result.RowsAffected()
			if err != nil {
				return Error.Wrap(err)
			}
			if count > 0 {
				valueUpdated = true
			}
		}
	} else {
		queryMatch := `SELECT * FROM pathdata WHERE bucket = ? AND fullpath = ?`
		result, err := client.sqliteConn.Exec(queryMatch, []byte(bucket), []byte(key))
		if err != nil {
			return Error.Wrap(err)
		}
		count, err := result.RowsAffected()
		if err != nil {
			return Error.Wrap(err)
		}
		if count > 0 {
			keyPresent = true
		}
		if keyPresent {
			query := `
				UPDATE pathdata
				SET metadata = ?
				WHERE bucket = ? AND fullpath = ? AND metadata = ?
			`
			result, err = client.sqliteConn.Exec(query, []byte(bucket), []byte(key), []byte(oldValue), []byte(newValue))
			if err != nil {
				return Error.Wrap(err)
			}
			count, err = result.RowsAffected()
			if err != nil {
				return Error.Wrap(err)
			}
			if count > 0 {
				valueUpdated = true
			}
		}
	}

	if !keyPresent {
		return storage.ErrKeyNotFound.New("%q", key)
	}
	if !valueUpdated {
		return storage.ErrValueChanged.New("%q", key)
	}
	return nil
}

// Close closes the db connection
func (client *Client) Close() error {
	return client.sqliteConn.Close()
}

type orderedSqliteIterator struct {
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
func (opi *orderedSqliteIterator) Next(ctx context.Context, item *storage.ListItem) bool {
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

func (opi *orderedSqliteIterator) doNextQuery(ctx context.Context) (_ *sql.Rows, err error) {
	defer mon.Task()(&ctx)(&err)
	start := opi.lastKeySeen
	if start == nil {
		start = opi.opts.First
	}
	var query string
	// TODO: figure out how to handle this with sqlite
	// if !opi.opts.Recurse {
	// 	if opi.opts.Reverse {
	// 		query = "SELECT p, m FROM list_directory_reverse(?, ?, ?, $4) ld(p, m)"
	// 	} else {
	// 		query = "SELECT p, m FROM list_directory(?, ?, ?, $4) ld(p, m)"
	// 	}
	// } else {
	startCmp := ">="
	orderDir := ""
	if opi.opts.Reverse {
		startCmp = "<="
		orderDir = " DESC"
	}
	query = fmt.Sprintf(`
			SELECT fullpath, metadata
			  FROM pathdata
			 WHERE bucket = ?
			   AND (? = '' OR fullpath >= ?)
			   AND (? = '' OR fullpath < bytea_increment(?))
			   AND (? = '' OR fullpath %s ?)
			 ORDER BY fullpath%s
			 LIMIT ?
		`, startCmp, orderDir)
	return opi.client.sqliteConn.Query(query,
		[]byte(opi.bucket),      // bucket = ?
		[]byte(opi.opts.Prefix), // AND (? = ''
		[]byte(opi.opts.Prefix), // OR fullpath >= ?)
		[]byte(opi.opts.Prefix), // (? = ''
		// TODO: figure out how to translate the postgres function bytea_increment to sqlite?
		[]byte(opi.opts.Prefix), // bytea_increment($2::BYTEA))
		[]byte(start),           //
		[]byte(start),           //
		opi.batchSize+1,         // limit
	)
}

func (opi *orderedSqliteIterator) Close() error {
	return errs.Combine(opi.errEncountered, opi.curRows.Close())
}

func newOrderedSqliteIterator(ctx context.Context, sqliteConn *Client, opts storage.IterateOptions, batchSize int) (_ *orderedSqliteIterator, err error) {
	defer mon.Task()(&ctx)(&err)
	if opts.Prefix == nil {
		opts.Prefix = storage.Key("")
	}
	if opts.First == nil {
		opts.First = storage.Key("")
	}
	opi := &orderedSqliteIterator{
		client:    sqliteConn,
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