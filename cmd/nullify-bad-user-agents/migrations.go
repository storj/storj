// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	pgx "github.com/jackc/pgx/v4"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/private/dbutil/cockroachutil"
)

// MigrateTables runs the migration for each table.
func MigrateTables(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	err = MigrateUsers(ctx, log, conn, config)
	if err != nil {
		return errs.New("error migrating users: %w", err)
	}
	err = MigrateProjects(ctx, log, conn, config)
	if err != nil {
		return errs.New("error migrating projects: %w", err)
	}
	err = MigrateAPIKeys(ctx, log, conn, config)
	if err != nil {
		return errs.New("error migrating api_keys: %w", err)
	}
	err = MigrateBucketMetainfos(ctx, log, conn, config)
	if err != nil {
		return errs.New("error migrating bucket_metainfos: %w", err)
	}
	err = MigrateValueAttributions(ctx, log, conn, config)
	if err != nil {
		return errs.New("error migrating value_attributions: %w", err)
	}
	return nil
}

// MigrateUsers updates the user_agent column to corresponding Partners.Names or partner_id if applicable.
func MigrateUsers(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	// We select the next id then use limit as an offset which actually gives us limit+1 rows.
	offset := config.Limit - 1

	startID := []byte{}
	nextID := []byte{}
	var total int
	more := true

	// select the next ID after startID and offset the result. We will update relevant rows from the range of startID to nextID.
	_, err = conn.Prepare(ctx, "select-for-update-users", "SELECT id FROM users WHERE id > $1 ORDER BY id OFFSET $2 LIMIT 1")
	if err != nil {
		return errs.New("could not prepare select query")
	}

	// update range from startID to nextID. Return count of updated rows to log progress.
	_, err = conn.Prepare(ctx, "update-limited-users", `
		WITH updated as (
			UPDATE users
			SET user_agent = NULL
			WHERE users.id > $1 AND users.id <= $2
				AND user_agent = partner_id
			RETURNING 1
		)
		SELECT count(*)
		FROM updated;
	`)
	if err != nil {
		return errs.New("could not prepare update statement: %w", err)
	}

	for {
		row := conn.QueryRow(ctx, "select-for-update-users", startID, offset)
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
				row = conn.QueryRow(ctx, "update-limited-users", startID, nextID)
			} else {
				// if !more then the select statement reached the end of the table. Update to the end of the table.
				row = conn.QueryRow(ctx, `
					WITH updated as (
						UPDATE users
						SET user_agent = NULL
						WHERE users.id > $1
							AND user_agent = partner_id
						RETURNING 1
					)
					SELECT count(*)
					FROM updated;
				`, startID,
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
func MigrateProjects(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	// We select the next id then use limit as an offset which actually gives us limit+1 rows.
	offset := config.Limit - 1

	startID := []byte{}
	nextID := []byte{}
	var total int
	more := true

	// select the next ID after startID and offset the result. We will update relevant rows from the range of startID to nextID.
	_, err = conn.Prepare(ctx, "select-for-update-projects", "SELECT id FROM projects WHERE id > $1 ORDER BY id OFFSET $2 LIMIT 1")
	if err != nil {
		return errs.New("could not prepare select query: %w", err)
	}

	// update range from startID to nextID. Return count of updated rows to log progress.
	_, err = conn.Prepare(ctx, "update-limited-projects", `
		WITH updated as (
			UPDATE projects
			SET user_agent = NULL
			WHERE projects.id > $1 AND projects.id <= $2
				AND user_agent = partner_id
			RETURNING 1
		)
		SELECT count(*)
		FROM updated;
	`)
	if err != nil {
		return errs.New("could not prepare update statement")
	}

	for {
		row := conn.QueryRow(ctx, "select-for-update-projects", startID, offset)
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
				row = conn.QueryRow(ctx, "update-limited-projects", startID, nextID)
			} else {
				// if !more then the select statement reached the end of the table. Update to the end of the table.
				row = conn.QueryRow(ctx, `
					WITH updated as (
						UPDATE projects
						SET user_agent = NULL
						WHERE projects.id > $1
							AND user_agent = partner_id
						RETURNING 1
					)
					SELECT count(*)
					FROM updated;
				`, startID,
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
func MigrateAPIKeys(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	// We select the next id then use limit as an offset which actually gives us limit+1 rows.
	offset := config.Limit - 1

	startID := []byte{}
	nextID := []byte{}
	var total int
	more := true

	// select the next ID after startID and offset the result. We will update relevant rows from the range of startID to nextID.
	_, err = conn.Prepare(ctx, "select-for-update-api-keys", "SELECT id FROM api_keys WHERE id > $1 ORDER BY id OFFSET $2 LIMIT 1")
	if err != nil {
		return errs.New("could not prepare select query: %w", err)
	}

	// update range from startID to nextID. Return count of updated rows to log progress.
	_, err = conn.Prepare(ctx, "update-limited-api-keys", `
		WITH updated as (
			UPDATE api_keys
			SET user_agent = NULL
			WHERE api_keys.id > $1 AND api_keys.id <= $2
				AND user_agent = partner_id
			RETURNING 1
		)
		SELECT count(*)
		FROM updated;
	`)
	if err != nil {
		return errs.New("could not prepare update statement")
	}

	for {
		row := conn.QueryRow(ctx, "select-for-update-api-keys", startID, offset)
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
				row = conn.QueryRow(ctx, "update-limited-api-keys", startID, nextID)
			} else {
				// if !more then the select statement reached the end of the table. Update to the end of the table.
				row = conn.QueryRow(ctx, `
					WITH updated as (
						UPDATE api_keys
						SET user_agent = NULL
						WHERE api_keys.id > $1
							AND user_agent = partner_id
						RETURNING 1
					)
					SELECT count(*)
					FROM updated;
				`, startID,
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
func MigrateBucketMetainfos(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	// We select the next id then use limit as an offset which actually gives us limit+1 rows.
	offset := config.Limit - 1

	startID := []byte{}
	nextID := []byte{}
	var total int
	more := true

	// select the next ID after startID and offset the result. We will update relevant rows from the range of startID to nextID.
	_, err = conn.Prepare(ctx, "select-for-update-bucket-metainfos", "SELECT id FROM bucket_metainfos WHERE id > $1 ORDER BY id OFFSET $2 LIMIT 1")
	if err != nil {
		return errs.New("could not prepare select query: %w", err)
	}

	// update range from startID to nextID. Return count of updated rows to log progress.
	_, err = conn.Prepare(ctx, "update-limited-bucket-metainfos", `
		WITH updated as (
			UPDATE bucket_metainfos
			SET user_agent = NULL
			WHERE bucket_metainfos.id > $1 AND bucket_metainfos.id <= $2
				AND user_agent = partner_id
			RETURNING 1
		)
		SELECT count(*)
		FROM updated;
	`)
	if err != nil {
		return errs.New("could not prepare update statement")
	}

	for {
		row := conn.QueryRow(ctx, "select-for-update-bucket-metainfos", startID, offset)
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
				row = conn.QueryRow(ctx, "update-limited-bucket-metainfos", startID, nextID)
			} else {
				// if !more then the select statement reached the end of the table. Update to the end of the table.
				row = conn.QueryRow(ctx, `
					WITH updated as (
						UPDATE bucket_metainfos
						SET user_agent = NULL
						WHERE bucket_metainfos.id > $1
							AND user_agent = partner_id
						RETURNING 1
					)
					SELECT count(*)
					FROM updated;
				`, startID,
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
func MigrateValueAttributions(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	// We select the next id then use limit as an offset which actually gives us limit+1 rows.
	offset := config.Limit - 1

	startProjectID := []byte{}
	startBucket := []byte{}
	nextProjectID := []byte{}
	nextBucket := []byte{}
	var total int
	more := true

	// select the next ID after startID and offset the result. We will update relevant rows from the range of startID to nextID.
	_, err = conn.Prepare(ctx, "select-for-update-value-attributions", `
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
		return errs.New("could not prepare select query: %w", err)
	}

	// update range from startID to nextID. Return count of updated rows to log progress.
	_, err = conn.Prepare(ctx, "update-limited-value-attributions", `
		WITH updated as (
			UPDATE value_attributions
			SET user_agent = NULL
			WHERE user_agent = partner_id
				AND (
					value_attributions.project_id > $1
					OR (
						value_attributions.project_id = $1 AND value_attributions.bucket_name > $3
					)
				)
				AND (
					value_attributions.project_id < $2
					OR (
						value_attributions.project_id = $2 AND value_attributions.bucket_name <= $4
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
		row := conn.QueryRow(ctx, "select-for-update-value-attributions", startProjectID, startBucket, offset)
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
				row = conn.QueryRow(ctx, "update-limited-value-attributions", startProjectID, nextProjectID, startBucket, nextBucket)
			} else {
				// if !more then the select statement reached the end of the table. Update to the end of the table.
				row = conn.QueryRow(ctx, `
					WITH updated as (
						UPDATE value_attributions
						SET user_agent = NULL
						WHERE user_agent = partner_id
							AND (
								value_attributions.project_id > $1
								OR (
									value_attributions.project_id = $1 AND value_attributions.bucket_name > $2
								)
							)
						RETURNING 1
					)
					SELECT count(*)
					FROM updated;
				`, startProjectID, startBucket,
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
