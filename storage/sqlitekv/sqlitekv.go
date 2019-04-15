// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:generate dbx.v1 golang -d sqlite3 sqlitekv.dbx .

package sqlitekv

import (
	"context"
	"github.com/zeebo/errs"

	_ "github.com/mattn/go-sqlite3"

	"storj.io/storj/storage"
)

type SqliteKV struct {
	DB *DB
}

// New opens a new SqliteKV database connection.
func New(path string) (storage.KeyValueStore, error) {
	db, err := Open("sqlite3", path)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	sqliteDB := &SqliteKV{
		DB: db,
	}
	return sqliteDB, nil
}

func (db SqliteKV) Get(key storage.Key) (storage.Value, error) {
	// TODO: should we add contexts to the key-value store interface?
}

func (db SqliteKV) Put(key storage.Key, value storage.Value) error {
	// TODO: should we add contexts to the key-value store interface?
	// TODO: protobuf (de)serialization?
	_, err := db.DB.Update_Item_By_Key(context.Background(), Item_Key(key), Item_Update_Fields{Item_Value(value)})
	return err
}

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
	return values, findErrs.Err()
}

func (db SqliteKV) Delete(key storage.Key) error {
	// TODO: should we add contexts to the key-value store interface?
}

func (db SqliteKV) List(start storage.Key, limit int) (storage.Keys, error) {
	// TODO: should we add contexts to the key-value store interface?
}

func (db SqliteKV) Iterate(opts storage.IterateOptions, fn func(storage.Iterator) error) error {
	// TODO: should we add contexts to the key-value store interface?
}

func (db SqliteKV) Close() error {
	return db.Close()
}
