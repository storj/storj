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
	_, err = conn.Prepare(ctx, "select-for-update", "SELECT id FROM users WHERE id > $1 ORDER BY id OFFSET $2 LIMIT 1")
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

// MigrateProjects updates the user_agent column to corresponding PartnerInfo.Names or partner_id if applicable.
func MigrateProjects(ctx context.Context, log *zap.Logger, conn *pgx.Conn, p *Partners, offset int) (err error) {
	defer mon.Task()(&ctx)(&err)

	startID := []byte{}
	nextID := []byte{}
	var total int
	more := true

	// select the next ID after startID and offset the result. We will update relevant rows from the range of startID to nextID.
	_, err = conn.Prepare(ctx, "select-for-update", "SELECT id FROM projects WHERE id > $1 ORDER BY id OFFSET $2 LIMIT 1")
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
			UPDATE projects
			SET user_agent = (
				CASE
					WHEN partner_id IN (SELECT id FROM partners)
						THEN (SELECT name FROM partners WHERE partner_id = partners.id)
					ELSE partner_id
				END
			)
			FROM partners
			WHERE projects.id > $3 AND projects.id <= $4
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
				return errs.New("unable to select row for update from projects: %w", err)
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
						UPDATE projects
						SET user_agent = (
							CASE
								WHEN partner_id IN (SELECT id FROM partners)
									THEN (SELECT name FROM partners WHERE partner_id = partners.id)
								ELSE partner_id
							END
						)
						FROM partners
						WHERE projects.id > $3
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
				return errs.New("updating projects %w", err)
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
	log.Info("projects migration complete", zap.Int("total rows updated", total))
	return nil
}

// MigrateAPIKeys updates the user_agent column to corresponding PartnerInfo.Names or partner_id if applicable.
func MigrateAPIKeys(ctx context.Context, log *zap.Logger, conn *pgx.Conn, p *Partners, offset int) (err error) {
	defer mon.Task()(&ctx)(&err)

	startID := []byte{}
	nextID := []byte{}
	var total int
	more := true

	// select the next ID after startID and offset the result. We will update relevant rows from the range of startID to nextID.
	_, err = conn.Prepare(ctx, "select-for-update", "SELECT id FROM api_keys WHERE id > $1 ORDER BY id OFFSET $2 LIMIT 1")
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
			UPDATE api_keys
			SET user_agent = (
				CASE
					WHEN partner_id IN (SELECT id FROM partners)
						THEN (SELECT name FROM partners WHERE partner_id = partners.id)
					ELSE partner_id
				END
			)
			FROM partners
			WHERE api_keys.id > $3 AND api_keys.id <= $4
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
				return errs.New("unable to select row for update from api_keys: %w", err)
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
						UPDATE api_keys
						SET user_agent = (
							CASE
								WHEN partner_id IN (SELECT id FROM partners)
									THEN (SELECT name FROM partners WHERE partner_id = partners.id)
								ELSE partner_id
							END
						)
						FROM partners
						WHERE api_keys.id > $3
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
				return errs.New("updating api_keys %w", err)
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
	log.Info("api_keys migration complete", zap.Int("total rows updated", total))
	return nil
}

// MigrateBucketMetainfos updates the user_agent column to corresponding Partners.Names or partner_id if applicable.
func MigrateBucketMetainfos(ctx context.Context, log *zap.Logger, conn *pgx.Conn, p *Partners, offset int) (err error) {
	defer mon.Task()(&ctx)(&err)

	startID := []byte{}
	nextID := []byte{}
	var total int
	more := true

	// select the next ID after startID and offset the result. We will update relevant rows from the range of startID to nextID.
	_, err = conn.Prepare(ctx, "select-for-update", "SELECT id FROM bucket_metainfos WHERE id > $1 ORDER BY id OFFSET $2 LIMIT 1")
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
			UPDATE bucket_metainfos
			SET user_agent = (
				CASE
					WHEN partner_id IN (SELECT id FROM partners)
						THEN (SELECT name FROM partners WHERE partner_id = partners.id)
					ELSE partner_id
				END
			)
			FROM partners
			WHERE bucket_metainfos.id > $3 AND bucket_metainfos.id <= $4
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
				return errs.New("unable to select row for update from bucket_metainfos: %w", err)
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
						UPDATE bucket_metainfos
						SET user_agent = (
							CASE
								WHEN partner_id IN (SELECT id FROM partners)
									THEN (SELECT name FROM partners WHERE partner_id = partners.id)
								ELSE partner_id
							END
						)
						FROM partners
						WHERE bucket_metainfos.id > $3
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
				return errs.New("updating bucket_metainfos %w", err)
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
	log.Info("bucket_metainfos migration complete", zap.Int("total rows updated", total))
	return nil
}

// MigrateValueAttributions updates the user_agent column to corresponding Partners.Names or partner_id if applicable.
func MigrateValueAttributions(ctx context.Context, log *zap.Logger, conn *pgx.Conn, p *Partners, offset int) (err error) {
	defer mon.Task()(&ctx)(&err)

	startProjectID := []byte{}
	startBucket := []byte{}
	nextProjectID := []byte{}
	nextBucket := []byte{}
	var total int
	more := true

	// select the next ID after startID and offset the result. We will update relevant rows from the range of startID to nextID.
	_, err = conn.Prepare(ctx, "select-for-update", `
		SELECT project_id, bucket_name
		FROM value_attributions
		WHERE project_id > $1
			OR (
				project_id = $1 AND bucket_name > $2
			)
		ORDER BY project_id, bucket_name
		OFFSET $3
		LIMIT 1
	`)
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
			UPDATE value_attributions
			SET user_agent = (
				CASE
					WHEN partner_id IN (SELECT id FROM partners)
						THEN (SELECT name FROM partners WHERE partner_id = partners.id)
					ELSE partner_id
				END
			)
			FROM partners
			WHERE partner_id IS NOT NULL
				AND user_agent IS NULL
				AND (
					value_attributions.project_id > $3
					OR (
						value_attributions.project_id = $3 AND value_attributions.bucket_name > $5
					)
				)
				AND (
					value_attributions.project_id < $4
					OR (
						value_attributions.project_id = $4 AND value_attributions.bucket_name <= $6
					)
				)
			RETURNING 1
		)
		SELECT count(*)
		FROM updated;
	`)
	if err != nil {
		return errs.New("could not prepare update statement")
	}

	for {
		row := conn.QueryRow(ctx, "select-for-update", startProjectID, startBucket, offset)
		err = row.Scan(&nextProjectID, &nextBucket)
		if err != nil {
			if errs.Is(err, pgx.ErrNoRows) {
				more = false
			} else {
				return errs.New("unable to select row for update from value_attributions: %w", err)
			}
		}
		var updated int
		for {
			var row pgx.Row
			if more {
				row = conn.QueryRow(ctx, "update-limited", pgutil.UUIDArray(p.UUIDs), pgutil.ByteaArray(p.Names), startProjectID, nextProjectID, startBucket, nextBucket)
			} else {
				// if !more then the select statement reached the end of the table. Update to the end of the table.
				row = conn.QueryRow(ctx, `
					WITH partners AS (
						SELECT unnest($1::bytea[]) as id,
							unnest($2::bytea[]) as name
					),
					updated as (
						UPDATE value_attributions
						SET user_agent = (
							CASE
								WHEN partner_id IN (SELECT id FROM partners)
									THEN (SELECT name FROM partners WHERE partner_id = partners.id)
								ELSE partner_id
							END
						)
						FROM partners
						WHERE partner_id IS NOT NULL
							AND user_agent IS NULL
							AND (
								value_attributions.project_id > $3
								OR (
									value_attributions.project_id = $3 AND value_attributions.bucket_name > $4
								)
							)
						RETURNING 1
					)
					SELECT count(*)
					FROM updated;
				`, pgutil.UUIDArray(p.UUIDs), pgutil.ByteaArray(p.Names), startProjectID, startBucket,
				)
			}
			err := row.Scan(&updated)
			if err != nil {
				if cockroachutil.NeedsRetry(err) {
					continue
				} else if errs.Is(err, pgx.ErrNoRows) {
					break
				}
				return errs.New("updating value_attributions %w", err)
			}
			break
		}

		total += updated
		if !more {
			log.Info("batch update complete", zap.Int("rows updated", updated))
			break
		}
		log.Info("batch update complete", zap.Int("rows updated", updated), zap.Binary("last project id", nextProjectID), zap.String("last bucket name", string(nextBucket)))
		startProjectID = nextProjectID
		startBucket = nextBucket
	}
	log.Info("value_attributions migration complete", zap.Int("total rows updated", total))
	return nil
}
