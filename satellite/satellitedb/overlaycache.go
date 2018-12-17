// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"errors"

	"storj.io/storj/pkg/utils"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
	"storj.io/storj/storage"
)

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

	tx, err := o.db.Open(o.ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	_, err = tx.Get_OverlayCacheNode_By_Key(o.ctx, dbx.OverlayCacheNode_Key(key))
	if err != nil {
		_, err = tx.Create_OverlayCacheNode(
			o.ctx,
			dbx.OverlayCacheNode_Key(key),
			dbx.OverlayCacheNode_Value(value),
		)
		if err != nil {
			return Error.Wrap(utils.CombineErrors(err, tx.Rollback()))
		}
	} else {
		updateFields := dbx.OverlayCacheNode_Update_Fields{}
		updateFields.Value = dbx.OverlayCacheNode_Value(value)
		_, err := tx.Update_OverlayCacheNode_By_Key(
			o.ctx,
			dbx.OverlayCacheNode_Key(key),
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

	node, err := o.db.Get_OverlayCacheNode_By_Key(o.ctx, dbx.OverlayCacheNode_Key(key))
	if err != nil {
		return nil, err
	}
	return node.Value, nil
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
	_, err := o.db.Delete_OverlayCacheNode_By_Key(o.ctx, dbx.OverlayCacheNode_Key(key))
	return err
}

func (o *overlaycache) List(start storage.Key, limit int) (keys storage.Keys, err error) {
	rows, err := o.db.Limited_OverlayCacheNode_By_Key_GreaterOrEqual(o.ctx, dbx.OverlayCacheNode_Key(start), limit, 0)
	if err != nil {
		return []storage.Key{}, err
	}

	keys = make([]storage.Key, len(rows))
	for i, row := range rows {
		keys[i] = row.Key
	}

	return keys, nil
}

// ReverseList lists all keys in revers order
func (o *overlaycache) ReverseList(start storage.Key, limit int) (storage.Keys, error) {
	return nil, errors.New("not implemented")
}

// Iterate iterates over items based on opts
func (o *overlaycache) Iterate(opts storage.IterateOptions, fn func(storage.Iterator) error) error {
	return errors.New("not implemented")
}

// Close closes the store
func (o *overlaycache) Close() error {
	return errors.New("not implemented")
}
