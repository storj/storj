// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"crypto/sha256"

	pgx "github.com/jackc/pgx/v4"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/private/dbutil/cockroachutil"
	"storj.io/private/dbutil/pgutil"
)

// GenerateMissingSalt updates the salt column in projects table to add SHA-256 hash of the project ID where salt is NULL.
func GenerateMissingSalt(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	lastID := []byte{}
	var total int
	var rowsFound bool

	for {
		rowsFound = false
		idsToUpdate := struct {
			ids  [][]byte
			salt [][]byte
		}{}
		err = func() error {
			rows, err := conn.Query(ctx, `
			SELECT id FROM projects
			WHERE salt IS NULL
			AND id > $1
			ORDER BY id
			LIMIT $2
			`, lastID, config.Limit)
			if err != nil {
				return errs.New("error select ids for update: %w", err)
			}
			defer rows.Close()

			for rows.Next() {
				rowsFound = true
				var id []byte
				err = rows.Scan(&id)
				if err != nil {
					return errs.New("error scanning results from select: %w", err)
				}

				idHash := sha256.Sum256(id)
				salt := idHash[:]
				idsToUpdate.salt = append(idsToUpdate.salt, salt)
				idsToUpdate.ids = append(idsToUpdate.ids, id)
				lastID = id
			}

			return rows.Err()
		}()
		if err != nil {
			return err
		}
		if !rowsFound {
			break
		}

		var updated int
		for {
			row := conn.QueryRow(ctx,
				`WITH to_update AS (
					SELECT unnest($1::bytea[]) as id,
					unnest($2::bytea[]) as salt
				),
				updated as (
					UPDATE projects
					SET salt = to_update.salt
					FROM to_update
					WHERE projects.id = to_update.id
					RETURNING 1
				)
				SELECT count(*)
				FROM updated;
			`, pgutil.ByteaArray(idsToUpdate.ids), pgutil.ByteaArray(idsToUpdate.salt))
			err := row.Scan(&updated)
			if err != nil {
				if cockroachutil.NeedsRetry(err) {
					continue
				} else if errs.Is(err, pgx.ErrNoRows) {
					break
				}
				return errs.New("error updating projects %w", err)
			}
			break
		}
		total += updated
		log.Info("batch update complete", zap.Int("rows updated", updated), zap.Binary("last id", lastID))
	}
	log.Info("projects migration complete", zap.Int("total rows updated", total))
	return nil
}
