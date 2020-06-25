// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"fmt"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/cache"
	"storj.io/storj/satellite/satellitedb/dbx"
)

type revocationDB struct {
	db      *satelliteDB
	lru     *cache.ExpiringLRU
	methods dbx.Methods
}

// Revoke will revoke the supplied tail
func (db *revocationDB) Revoke(ctx context.Context, tail []byte, apiKeyID []byte) error {
	return errs.Wrap(db.methods.CreateNoReturn_Revocation(ctx, dbx.Revocation_Revoked(tail), dbx.Revocation_ApiKeyId(apiKeyID)))
}

// Check will check whether any of the supplied tails have been revoked
func (db *revocationDB) Check(ctx context.Context, tails [][]byte) (bool, error) {
	numTails := len(tails)
	if numTails == 0 {
		return false, errs.New("Empty list of tails")
	}

	finalTail := tails[numTails-1]

	val, err := db.lru.Get(string(finalTail), func() (interface{}, error) {
		const query = "select exists(select 1 from revocations where revoked in (%s))"

		var (
			tailQuery, comma string
			tailsForQuery    = make([]interface{}, numTails)
			revoked          bool
		)

		for i, tail := range tails {
			if i == 1 {
				comma = ","
			}
			tailQuery += fmt.Sprintf("%s$%d", comma, i+1)
			tailsForQuery[i] = tail
		}

		row := db.db.QueryRowContext(ctx, fmt.Sprintf(query, tailQuery), tailsForQuery...)
		err := row.Scan(&revoked)
		if err != nil {
			return nil, err
		}

		return revoked, nil
	})
	if err != nil {
		return false, errs.Wrap(err)
	}

	revoked, ok := val.(bool)
	if !ok {
		return false, errs.New("Revoked not a bool")
	}

	return revoked, nil
}
