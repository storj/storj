// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	pgx "github.com/jackc/pgx/v4"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/private/dbutil/cockroachutil"
	"storj.io/private/dbutil/pgutil"
)

// MigrateUsers updates the user_agent column to corresponding Partners.Names or partner_id if applicable.
func MigrateUsers(ctx context.Context, log *zap.Logger, conn *pgx.Conn, p *Partners, offset int) (err error) {
	defer mon.Task()(&ctx)(&err)

	startID := []byte{}
	nextID := []byte{}
	var total int
	more := true

	// select the next ID after startID and offset the result. We will update relevant rows from the range of startID to nextID.
	_, err = conn.Prepare(ctx, "select-for-update", "SELECT id FROM users WHERE id > $1 ORDER BY id OFFSET $2")
	if err != nil {
		return errs.New("could not prepare select query")
	}

	// update range from startID to nextID. Return count of updated rows to log progress.
	_, err = conn.Prepare(ctx, "update-limited", `
		WITH partners AS (
			SELECT unnest($1::bytea[]) as id,
				unnest($2::bytea[]) as name
		),
		updated as (
			UPDATE users
			SET user_agent = (
				CASE
					WHEN partner_id IN (SELECT id FROM partners)
						THEN (SELECT name FROM partners WHERE partner_id = partners.id)
					ELSE partner_id
				END
			)
			FROM partners
			WHERE users.id > $3 AND users.id <= $4
				AND partner_id IS NOT NULL
				AND user_agent IS NULL
			RETURNING 1
		)
		SELECT count(*)
		FROM updated;
	`)
	if err != nil {
		return errs.New("could not prepare update statement")
	}

	for {
		row := conn.QueryRow(ctx, "select-for-update", startID, offset)
		err = row.Scan(&nextID)
		if err != nil {
			if errs.Is(err, pgx.ErrNoRows) {
				more = false
			} else {
				return errs.New("unable to select row for update from users: %w", err)
			}
		}

		var updated int
		for {
			var row pgx.Row
			if more {
				row = conn.QueryRow(ctx, "update-limited", pgutil.UUIDArray(p.UUIDs), pgutil.ByteaArray(p.Names), startID, nextID)
			} else {
				// if !more then the select statement reached the end of the table. Update to the end of the table.
				row = conn.QueryRow(ctx, `
					WITH partners AS (
						SELECT unnest($1::bytea[]) as id,
							unnest($2::bytea[]) as name
					),
					updated as (
						UPDATE users
						SET user_agent = (
							CASE
								WHEN partner_id IN (SELECT id FROM partners)
									THEN (SELECT name FROM partners WHERE partner_id = partners.id)
								ELSE partner_id
							END
						)
						FROM partners
						WHERE users.id > $3
							AND partner_id IS NOT NULL
							AND user_agent IS NULL
						RETURNING 1
					)
					SELECT count(*)
					FROM updated;
				`, pgutil.UUIDArray(p.UUIDs), pgutil.ByteaArray(p.Names), startID,
				)
			}
			err := row.Scan(&updated)
			if err != nil {
				if cockroachutil.NeedsRetry(err) {
					continue
				} else if errs.Is(err, pgx.ErrNoRows) {
					break
				}
				return errs.New("updating users %w", err)
			}
			break
		}

		total += updated
		if !more {
			log.Info("batch update complete", zap.Int("rows updated", updated))
			break
		}
		log.Info("batch update complete", zap.Int("rows updated", updated), zap.Binary("last id", nextID))
		startID = nextID
	}
	log.Info("users migration complete", zap.Int("total rows updated", total))
	return nil
}
