// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"fmt"

	"github.com/zeebo/errs"

	"storj.io/common/lrucache"
	"storj.io/storj/satellite/satellitedb/dbx"
)

type revocationDB struct {
	db      *satelliteDB
	lru     *lrucache.ExpiringLRUOf[bool]
	methods dbx.Methods
}

// Revoke will revoke the supplied tail.
func (db *revocationDB) Revoke(ctx context.Context, tail []byte, apiKeyID []byte) error {
	return errs.Wrap(db.methods.CreateNoReturn_Revocation(ctx, dbx.Revocation_Revoked(tail), dbx.Revocation_ApiKeyId(apiKeyID)))
}

// Check will check whether any of the supplied tails have been revoked.
func (db *revocationDB) Check(ctx context.Context, tails [][]byte) (bool, error) {
	numTails := len(tails)
	if numTails == 0 {
		return false, errs.New("Empty list of tails")
	}

	// The finalTail is the last tail provided in the macaroon. We cache the
	// revocation status of this final tail so that, if this macaroon is used
	// again before the cache key expires, we do not have to check the database
	// again.
	finalTail := tails[numTails-1]

	revoked, err := db.lru.Get(ctx, string(finalTail), func() (bool, error) {
		const query = "SELECT EXISTS(SELECT 1 FROM revocations WHERE revoked IN (%s))"

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
			return false, err
		}

		return revoked, nil
	})
	if err != nil {
		return false, errs.Wrap(err)
	}

	return revoked, nil
}
