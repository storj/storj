// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	pgx "github.com/jackc/pgx/v5"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/retrydb"
)

// MigrateProjects updates all rows in the projects table, giving them a new UUID if they do not already have one.
func MigrateProjects(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	lastID := []byte{}
	var total int
	var rowsFound bool

	for {
		rowsFound = false
		idsToUpdate := struct {
			ids       [][]byte
			publicIDs [][]byte
		}{}
		err = func() error {
			rows, err := conn.Query(ctx, `
				SELECT id FROM projects
				WHERE id > $1
					AND (public_id IS NULL OR public_id = '\x00000000000000000000000000000000')
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
				publicID, err := uuid.New()
				if err != nil {
					return errs.New("error creating new uuid for public_id: %w", err)
				}
				idsToUpdate.ids = append(idsToUpdate.ids, id)
				idsToUpdate.publicIDs = append(idsToUpdate.publicIDs, publicID.Bytes())
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
			row := conn.QueryRow(ctx, `
				WITH to_update AS (
					SELECT unnest($1::bytea[]) as id,
						unnest($2::bytea[]) as public_id
				),
				updated as (
					UPDATE projects
					SET public_id = to_update.public_id
					FROM to_update
					WHERE projects.id = to_update.id
					RETURNING 1
				)
				SELECT count(*)
				FROM updated;
			`, pgutil.ByteaArray(idsToUpdate.ids), pgutil.ByteaArray(idsToUpdate.publicIDs),
			)
			err := row.Scan(&updated)
			if err != nil {
				if retrydb.ShouldRetry(err) {
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
