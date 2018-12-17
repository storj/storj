// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"bytes"
	"context"
	"encoding/hex"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/utils"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/storage"
)

var fullEmpty = make([]byte, 32)

type overlaycache struct {
	db  *dbx.DB
	ctx context.Context
}

func newOverlaycache(db *dbx.DB) *overlaycache {
	return &overlaycache{
		db:  db,
		ctx: context.Background(),
	}
}

func (o *overlaycache) Put(key storage.Key, value storage.Value) error {
	if key.IsZero() {
		return storage.ErrEmptyKey.New("")
	}

	tx, err := o.db.Begin()
	if err != nil {
		return Error.Wrap(err)
	}

	keyValue := hex.EncodeToString(key)
	_, err = o.Get(key)
	if err != nil {
		_, err = o.db.Create_OverlayCacheNode(
			o.ctx,
			dbx.OverlayCacheNode_Key(keyValue),
			dbx.OverlayCacheNode_Value(value),
		)
		if err != nil {
			return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}
	} else {
		updateFields := dbx.OverlayCacheNode_Update_Fields{}
		updateFields.Value = dbx.OverlayCacheNode_Value(value)
		_, err := o.db.Update_OverlayCacheNode_By_Key(
			o.ctx,
			dbx.OverlayCacheNode_Key(keyValue),
			updateFields,
		)
		if err != nil {
			return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}
	}
	return Error.Wrap(tx.Commit())
}

func (o *overlaycache) Get(key storage.Key) (storage.Value, error) {
	if key.IsZero() {
		return nil, storage.ErrEmptyKey.New("")
	}

	keyValue := hex.EncodeToString(key)
	node, err := o.db.Get_OverlayCacheNode_By_Key(o.ctx, dbx.OverlayCacheNode_Key(keyValue))
	if err != nil {
		return nil, err
	}
	return []byte(node.Value), nil
}

func (o *overlaycache) GetAll(keys storage.Keys) (storage.Values, error) {
	values := make([]storage.Value, len(keys))
	for i, key := range keys {
		value, err := o.Get(key)
		if err == nil {
			values[i] = value
		}
	}
	return values, nil
}

func (o *overlaycache) Delete(key storage.Key) error {
	keyValue := hex.EncodeToString(key)
	_, err := o.db.Delete_OverlayCacheNode_By_Key(o.ctx, dbx.OverlayCacheNode_Key(keyValue))
	return err
}

func (o *overlaycache) List(start storage.Key, limit int) (keys storage.Keys, err error) {
	// workaround for start key filled with zeros
	if bytes.Equal(start, fullEmpty) {
		start = storage.Key("")
	}

	if start.IsZero() {
		rows, err := o.db.Limited_OverlayCacheNode(o.ctx, limit, 0)
		if err != nil {
			return []storage.Key{}, err
		}

		keys = make([]storage.Key, len(rows))
		for _, row := range rows {
			decoded, err := hex.DecodeString(row.Key)
			if err != nil {
				return []storage.Key{}, err
			}
			keys = append(keys, decoded)
		}
	} else {
		prefixValue := hex.EncodeToString(start)
		rows, err := o.db.Query("SELECT key FROM overlay_cache_nodes n WHERE n.key LIKE $1 LIMIT $2", prefixValue+"%", limit)
		if err != nil {
			return []storage.Key{}, err
		}
		keys = make([]storage.Key, 0)
		for rows.Next() {
			var value string
			if err := rows.Scan(&value); err != nil {
				return nil, errs.Wrap(utils.CombineErrors(err, rows.Close()))
			}
			decoded, err := hex.DecodeString(value)
			if err != nil {
				return []storage.Key{}, err
			}
			keys = append(keys, decoded)
		}
	}

	return keys, nil
}

// ReverseList lists all keys in revers order
func (o *overlaycache) ReverseList(start storage.Key, limit int) (storage.Keys, error) {
	return nil, nil
}

// Iterate iterates over items based on opts
func (o *overlaycache) Iterate(opts storage.IterateOptions, fn func(storage.Iterator) error) error {
	return nil
}

// Close closes the store
func (o *overlaycache) Close() error {
	return nil
}
