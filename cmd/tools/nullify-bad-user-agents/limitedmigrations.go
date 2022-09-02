// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/jackc/pgtype"
	pgx "github.com/jackc/pgx/v4"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/private/dbutil/pgutil"
)

// MigrateTablesLimited runs the migration for each table with update count limits.
func MigrateTablesLimited(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	err = MigrateUsersLimited(ctx, log, conn, config)
	if err != nil {
		return errs.New("error migrating users: %w", err)
	}
	err = MigrateProjectsLimited(ctx, log, conn, config)
	if err != nil {
		return errs.New("error migrating projects: %w", err)
	}
	err = MigrateAPIKeysLimited(ctx, log, conn, config)
	if err != nil {
		return errs.New("error migrating api_keys: %w", err)
	}
	err = MigrateBucketMetainfosLimited(ctx, log, conn, config)
	if err != nil {
		return errs.New("error migrating bucket_metainfos: %w", err)
	}
	err = MigrateValueAttributionsLimited(ctx, log, conn, config)
	if err != nil {
		return errs.New("error migrating value_attributions: %w", err)
	}
	return nil
}

// MigrateUsersLimited updates the user_agent column to corresponding Partners.Names or partner_id if applicable for a limited number
// of rows.
func MigrateUsersLimited(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	log.Info("beginning users migration", zap.Int("max updates", config.MaxUpdates))

	// wrap select in anonymous function for deferred rows.Close()
	selected, err := func() (ids [][]byte, err error) {
		rows, err := conn.Query(ctx, `
			SELECT id FROM users 
			WHERE user_agent = partner_id
			LIMIT $1
		`, config.MaxUpdates)
		if err != nil {
			return nil, errs.New("selecting ids for update: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id pgtype.GenericBinary
			err = rows.Scan(&id)
			if err != nil {
				return nil, errs.New("scanning row: %w", err)
			}
			ids = append(ids, id.Bytes)
		}
		return ids, rows.Err()
	}()
	if err != nil {
		return err
	}

	row := conn.QueryRow(ctx, `
		WITH updated as (
			UPDATE users
			SET user_agent = NULL
			WHERE users.id IN (SELECT unnest($1::bytea[]))
			RETURNING 1
		)
		SELECT count(*)
		FROM updated
		`, pgutil.ByteaArray(selected),
	)
	var updated int
	err = row.Scan(&updated)
	if err != nil {
		return errs.New("error scanning results: %w", err)
	}
	log.Info("updated rows", zap.Int("count", updated))
	return nil
}

// MigrateProjectsLimited updates the user_agent column to corresponding Partners.Names or partner_id if applicable for a limited number
// of rows.
func MigrateProjectsLimited(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	log.Info("beginning projects migration", zap.Int("max updates", config.MaxUpdates))

	// wrap select in anonymous function for deferred rows.Close()
	selected, err := func() (ids [][]byte, err error) {
		rows, err := conn.Query(ctx, `
			SELECT id FROM projects 
			WHERE user_agent = partner_id
			LIMIT $1
		`, config.MaxUpdates)
		if err != nil {
			return nil, errs.New("selecting ids for update: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id pgtype.GenericBinary
			err = rows.Scan(&id)
			if err != nil {
				return nil, errs.New("scanning row: %w", err)
			}
			ids = append(ids, id.Bytes)
		}
		return ids, rows.Err()
	}()
	if err != nil {
		return err
	}

	row := conn.QueryRow(ctx, `
		WITH updated as (
			UPDATE projects
			SET user_agent = NULL
			WHERE projects.id IN (SELECT unnest($1::bytea[]))
			RETURNING 1
		)
		SELECT count(*)
		FROM updated
		`, pgutil.ByteaArray(selected),
	)
	var updated int
	err = row.Scan(&updated)
	if err != nil {
		return errs.New("error scanning results: %w", err)
	}
	log.Info("updated rows", zap.Int("count", updated))
	return nil
}

// MigrateAPIKeysLimited updates the user_agent column to corresponding Partners.Names or partner_id if applicable for a limited number
// of rows.
func MigrateAPIKeysLimited(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	log.Info("beginning api_keys migration", zap.Int("max updates", config.MaxUpdates))

	// wrap select in anonymous function for deferred rows.Close()
	selected, err := func() (ids [][]byte, err error) {
		rows, err := conn.Query(ctx, `
			SELECT id FROM api_keys 
			WHERE user_agent = partner_id
			LIMIT $1
		`, config.MaxUpdates)
		if err != nil {
			return nil, errs.New("selecting ids for update: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id pgtype.GenericBinary
			err = rows.Scan(&id)
			if err != nil {
				return nil, errs.New("scanning row: %w", err)
			}
			ids = append(ids, id.Bytes)
		}
		return ids, rows.Err()
	}()
	if err != nil {
		return err
	}

	row := conn.QueryRow(ctx, `
		WITH updated as (
			UPDATE api_keys
			SET user_agent = NULL
			WHERE api_keys.id IN (SELECT unnest($1::bytea[]))
			RETURNING 1
		)
		SELECT count(*)
		FROM updated
		`, pgutil.ByteaArray(selected),
	)
	var updated int
	err = row.Scan(&updated)
	if err != nil {
		return errs.New("error scanning results: %w", err)
	}
	log.Info("updated rows", zap.Int("count", updated))
	return nil
}

// MigrateBucketMetainfosLimited updates the user_agent column to corresponding Partners.Names or partner_id if applicable for a limited number
// of rows.
func MigrateBucketMetainfosLimited(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	log.Info("beginning bucket_metainfos migration", zap.Int("max updates", config.MaxUpdates))

	// wrap select in anonymous function for deferred rows.Close()
	selected, err := func() (ids [][]byte, err error) {
		rows, err := conn.Query(ctx, `
			SELECT id FROM bucket_metainfos
			WHERE user_agent = partner_id
			LIMIT $1
		`, config.MaxUpdates)
		if err != nil {
			return nil, errs.New("selecting ids for update: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var id pgtype.GenericBinary
			err = rows.Scan(&id)
			if err != nil {
				return nil, errs.New("scanning row: %w", err)
			}
			ids = append(ids, id.Bytes)
		}
		return ids, rows.Err()
	}()
	if err != nil {
		return err
	}

	row := conn.QueryRow(ctx, `
		WITH updated as (
			UPDATE bucket_metainfos
			SET user_agent = NULL
			WHERE bucket_metainfos.id IN (SELECT unnest($1::bytea[]))
			RETURNING 1
		)
		SELECT count(*)
		FROM updated
		`, pgutil.ByteaArray(selected),
	)
	var updated int
	err = row.Scan(&updated)
	if err != nil {
		return errs.New("error scanning results: %w", err)
	}
	log.Info("updated rows", zap.Int("count", updated))
	return nil
}

// MigrateValueAttributionsLimited updates the user_agent column to corresponding Partners.Names or partner_id if applicable for a limited number
// of rows.
func MigrateValueAttributionsLimited(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	log.Info("beginning value_attributions migration", zap.Int("max updates", config.MaxUpdates))

	// wrap select in anonymous function for deferred rows.Close()
	projects, buckets, err := func() (projectIDs [][]byte, buckets [][]byte, err error) {
		rows, err := conn.Query(ctx, `
			SELECT project_id, bucket_name FROM value_attributions
			WHERE user_agent = partner_id
			LIMIT $1
		`, config.MaxUpdates)
		if err != nil {
			return nil, nil, errs.New("selecting rows for update: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var projectID, bucketName pgtype.GenericBinary
			err = rows.Scan(&projectID, &bucketName)
			if err != nil {
				return nil, nil, errs.New("scanning row: %w", err)
			}
			projectIDs = append(projectIDs, projectID.Bytes)
			buckets = append(buckets, bucketName.Bytes)
		}
		return projectIDs, buckets, rows.Err()
	}()
	if err != nil {
		return err
	}

	row := conn.QueryRow(ctx, `
		WITH updated as (
			UPDATE value_attributions
			SET user_agent = NULL
			WHERE value_attributions.project_id IN (SELECT unnest($1::bytea[]))
				AND value_attributions.bucket_name IN (SELECT unnest($2::bytea[]))
				AND user_agent = partner_id
			RETURNING 1
		)
		SELECT count(*)
		FROM updated
		`, pgutil.ByteaArray(projects), pgutil.ByteaArray(buckets),
	)
	var updated int
	err = row.Scan(&updated)
	if err != nil {
		return errs.New("error scanning results: %w", err)
	}
	log.Info("updated rows", zap.Int("count", updated))
	return nil
}
