// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package sqlitekv

import (
	"database/sql"
	"github.com/zeebo/errs"

	_ "github.com/mattn/go-sqlite3"

	"storj.io/storj/storage"
)

type SqliteKV struct {
	DB *sql.DB
}

func New(path string) (storage.KeyValueStore, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	sqliteDB := &SqliteKV{
		DB: db,
	}



	return sqliteDB, nil
}

func (db SqliteKV) Get(key storage.Key) (storage.Value, error) {

}

func (db SqliteKV) Put(key storage.Key, value storage.Value) error {

}

func (db SqliteKV) GetAll(keys storage.Keys) (storage.Values, error) {

}

func (db SqliteKV) Delete(key storage.Key) error {

}

func (db SqliteKV) List(start storage.Key, limit int) (storage.Keys, error) {

}

func (db SqliteKV) Iterate(opts storage.IterateOptions, fn func(storage.Iterator) error) error {

}

func (db SqliteKV) Close() error {
	return db.Close()
}
