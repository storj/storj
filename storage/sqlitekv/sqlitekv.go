// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:generate dbx.v1 golang -d sqlite3 sqlitekv.dbx .

package sqlitekv

import (
	"context"

	_ "github.com/mattn/go-sqlite3"
	"github.com/zeebo/errs"

	"storj.io/storj/storage"
)

var ErrSqlitekv = errs.Class("sqlitekv error")

type SqliteKV struct {
	DB *DB
}

// New opens a new SqliteKV database connection.
func New(path string) (storage.KeyValueStore, error) {
	db, err := Open("sqlite3", path)
	if err != nil {
		return nil, ErrSqlitekv.Wrap(err)
	}
	sqliteDB := &SqliteKV{
		DB: db,
	}
	return sqliteDB, nil
}

// Get gets a value to store
func (db SqliteKV) Get(key storage.Key) (storage.Value, error) {
	// TODO: should we add contexts to the key-value store interface?
	row, err := db.DB.Find_Item_Value_By_Key(context.Background(), Item_Key(key))
	if err != nil {
		return nil, ErrSqlitekv.Wrap(err)
	}
	return row.Value, nil
}

// Put adds a value to store
func (db SqliteKV) Put(key storage.Key, value storage.Value) error {
	// TODO: should we add contexts to the key-value store interface?
	_, err := db.DB.Update_Item_By_Key(context.Background(), Item_Key(key), Item_Update_Fields{Item_Value(value)})
	return ErrSqlitekv.Wrap(err)
}

// GetAll gets all values from the store
func (db SqliteKV) GetAll(keys storage.Keys) (values storage.Values, _ error) {
	// TODO: should we add contexts to the key-value store interface?
	ctx := context.Background()
	findErrs := errs.Group{}
	for _, key := range keys {
		row, err := db.DB.Find_Item_Value_By_Key(ctx, Item_Key(key))
		if err != nil {
			findErrs.Add(err)
			continue
		}
		values = append(values, row.Value)
	}
	return values, ErrSqlitekv.Wrap(findErrs.Err())
}

// Delete deletes key and the value
func (db SqliteKV) Delete(key storage.Key) error {
	// TODO: should we add contexts to the key-value store interface?
	_, err := db.DB.Delete_Item_By_Key(context.Background(), Item_Key(key))
	return err
}

// List lists all keys starting from start and upto limit items
func (db SqliteKV) List(start storage.Key, limit int) (keys storage.Keys, _ error) {
	// TODO: should we add contexts to the key-value store interface?
	rows, err := db.DB.Limited_Item_Key_By_Key_GreaterOrEqual(
		context.Background(),
		Item_Key(start),
		limit,
		0,
	)
	if err != nil {
		return nil, ErrSqlitekv.Wrap(err)
	}
	for _, row := range rows {
		keys = append(keys, row.Key)
	}
	return keys, nil
}

// Iterate iterates over items based on opts
// NB: *Does not* implement reverse option is *always* recursive.
//		Prefix option does nothing.
func (db SqliteKV) Iterate(opts storage.IterateOptions, fn func(storage.Iterator) error) error {
	limit, offset := 1000, int64(0)

	for i := 1; ; i++ {
		// TODO: should we add contexts to the key-value store interface?
		rows, err := db.DB.Limited_Item_Key_Item_Value_By_Key_GreaterOrEqual(
			context.Background(),
			Item_Key(opts.First),
			limit,
			offset,
		)
		if err != nil {
			return ErrSqlitekv.Wrap(err)
		}

		var items storage.Items
		for _, row := range rows {
			items = append(items, storage.ListItem{
				Key:   row.Key,
				Value: row.Value,
				//IsPrefix:
			})
		}

		err = fn(&storage.StaticIterator{
			Items: items,
			Index: 0,
		})
		if err != nil {
			return ErrSqlitekv.Wrap(err)
		}

		if len(rows) < limit {
			break
		}
		offset += int64(limit)
	}
	return nil
}

// Close closes the store
func (db SqliteKV) Close() error {
	return db.Close()
}
