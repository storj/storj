// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/jackc/pgtype"
	pgx "github.com/jackc/pgx/v4"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/private/dbutil/pgutil"
)

// MigrateTablesLimited runs the migration for each table with update count limits.
func MigrateTablesLimited(ctx context.Context, log *zap.Logger, conn *pgx.Conn, p *Partners, config Config) (err error) {
	err = MigrateUsersLimited(ctx, log, conn, p, config)
	if err != nil {
		return errs.New("error migrating users: %w", err)
	}
	err = MigrateProjectsLimited(ctx, log, conn, p, config)
	if err != nil {
		return errs.New("error migrating projects: %w", err)
	}
	err = MigrateAPIKeysLimited(ctx, log, conn, p, config)
	if err != nil {
		return errs.New("error migrating api_keys: %w", err)
	}
	err = MigrateBucketMetainfosLimited(ctx, log, conn, p, config)
	if err != nil {
		return errs.New("error migrating bucket_metainfos: %w", err)
	}
	err = MigrateValueAttributionsLimited(ctx, log, conn, p, config)
	if err != nil {
		return errs.New("error migrating value_attributions: %w", err)
	}
	return nil
}

// MigrateUsersLimited updates the user_agent column to corresponding Partners.Names or partner_id if applicable for a limited number
// of rows.
func MigrateUsersLimited(ctx context.Context, log *zap.Logger, conn *pgx.Conn, p *Partners, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	log.Info("beginning users migration", zap.Int("max updates", config.MaxUpdates))

	// wrap select in anonymous function for deferred rows.Close()
	selected, err := func() (ids [][]byte, err error) {
		rows, err := conn.Query(ctx, `
			SELECT id FROM users 
			WHERE partner_id IS NOT NULL 
				AND user_agent IS NULL
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

	rows, err := conn.Query(ctx, `
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
			WHERE users.id IN (SELECT unnest($3::bytea[]))
			RETURNING partner_id, user_agent
		)
		SELECT partner_id, user_agent
		FROM updated
		`, pgutil.UUIDArray(p.UUIDs), pgutil.ByteaArray(p.Names), pgutil.ByteaArray(selected),
	)
	if err != nil {
		return errs.New("updating rows: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var partnerID, userAgent pgtype.GenericBinary
		err = rows.Scan(&partnerID, &userAgent)
		if err != nil {
			return errs.New("scanning row: %w", err)
		}
		id, err := uuid.FromBytes(partnerID.Bytes)
		if err != nil {
			return errs.New("error getting partnerID from bytes: %w", err)
		}
		log.Info("migrated row", zap.String("partner_id", id.String()), zap.String("user_agent", string(userAgent.Bytes)))
	}
	return rows.Err()
}

// MigrateProjectsLimited updates the user_agent column to corresponding Partners.Names or partner_id if applicable for a limited number
// of rows.
func MigrateProjectsLimited(ctx context.Context, log *zap.Logger, conn *pgx.Conn, p *Partners, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	log.Info("beginning projects migration", zap.Int("max updates", config.MaxUpdates))

	// wrap select in anonymous function for deferred rows.Close()
	selected, err := func() (ids [][]byte, err error) {
		rows, err := conn.Query(ctx, `
			SELECT id FROM projects 
			WHERE partner_id IS NOT NULL 
				AND user_agent IS NULL
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

	rows, err := conn.Query(ctx, `
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
			WHERE projects.id IN (SELECT unnest($3::bytea[]))
			RETURNING partner_id, user_agent
		)
		SELECT partner_id, user_agent
		FROM updated
		`, pgutil.UUIDArray(p.UUIDs), pgutil.ByteaArray(p.Names), pgutil.ByteaArray(selected),
	)
	if err != nil {
		return errs.New("updating rows: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var partnerID, userAgent pgtype.GenericBinary
		err = rows.Scan(&partnerID, &userAgent)
		if err != nil {
			return errs.New("scanning row: %w", err)
		}
		id, err := uuid.FromBytes(partnerID.Bytes)
		if err != nil {
			return errs.New("error getting partnerID from bytes: %w", err)
		}
		log.Info("migrated row", zap.String("partner_id", id.String()), zap.String("user_agent", string(userAgent.Bytes)))
	}
	return rows.Err()
}

// MigrateAPIKeysLimited updates the user_agent column to corresponding Partners.Names or partner_id if applicable for a limited number
// of rows.
func MigrateAPIKeysLimited(ctx context.Context, log *zap.Logger, conn *pgx.Conn, p *Partners, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	log.Info("beginning api_keys migration", zap.Int("max updates", config.MaxUpdates))

	// wrap select in anonymous function for deferred rows.Close()
	selected, err := func() (ids [][]byte, err error) {
		rows, err := conn.Query(ctx, `
			SELECT id FROM api_keys 
			WHERE partner_id IS NOT NULL 
				AND user_agent IS NULL
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

	rows, err := conn.Query(ctx, `
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
			WHERE api_keys.id IN (SELECT unnest($3::bytea[]))
			RETURNING partner_id, user_agent
		)
		SELECT partner_id, user_agent
		FROM updated
		`, pgutil.UUIDArray(p.UUIDs), pgutil.ByteaArray(p.Names), pgutil.ByteaArray(selected),
	)
	if err != nil {
		return errs.New("updating rows: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var partnerID, userAgent pgtype.GenericBinary
		err = rows.Scan(&partnerID, &userAgent)
		if err != nil {
			return errs.New("scanning row: %w", err)
		}
		id, err := uuid.FromBytes(partnerID.Bytes)
		if err != nil {
			return errs.New("error getting partnerID from bytes: %w", err)
		}
		log.Info("migrated row", zap.String("partner_id", id.String()), zap.String("user_agent", string(userAgent.Bytes)))
	}
	return rows.Err()
}

// MigrateBucketMetainfosLimited updates the user_agent column to corresponding Partners.Names or partner_id if applicable for a limited number
// of rows.
func MigrateBucketMetainfosLimited(ctx context.Context, log *zap.Logger, conn *pgx.Conn, p *Partners, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	log.Info("beginning bucket_metainfos migration", zap.Int("max updates", config.MaxUpdates))

	// wrap select in anonymous function for deferred rows.Close()
	selected, err := func() (ids [][]byte, err error) {
		rows, err := conn.Query(ctx, `
			SELECT id FROM bucket_metainfos
			WHERE partner_id IS NOT NULL 
				AND user_agent IS NULL
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

	rows, err := conn.Query(ctx, `
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
			WHERE bucket_metainfos.id IN (SELECT unnest($3::bytea[]))
			RETURNING partner_id, user_agent
		)
		SELECT partner_id, user_agent
		FROM updated
		`, pgutil.UUIDArray(p.UUIDs), pgutil.ByteaArray(p.Names), pgutil.ByteaArray(selected),
	)
	if err != nil {
		return errs.New("updating rows: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var partnerID, userAgent pgtype.GenericBinary
		err = rows.Scan(&partnerID, &userAgent)
		if err != nil {
			return errs.New("scanning row: %w", err)
		}
		id, err := uuid.FromBytes(partnerID.Bytes)
		if err != nil {
			return errs.New("error getting partnerID from bytes: %w", err)
		}
		log.Info("migrated row", zap.String("partner_id", id.String()), zap.String("user_agent", string(userAgent.Bytes)))
	}
	return rows.Err()
}

// MigrateValueAttributionsLimited updates the user_agent column to corresponding Partners.Names or partner_id if applicable for a limited number
// of rows.
func MigrateValueAttributionsLimited(ctx context.Context, log *zap.Logger, conn *pgx.Conn, p *Partners, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	log.Info("beginning value_attributions migration", zap.Int("max updates", config.MaxUpdates))

	// wrap select in anonymous function for deferred rows.Close()
	projects, buckets, err := func() (projectIDs [][]byte, buckets [][]byte, err error) {
		rows, err := conn.Query(ctx, `
			SELECT project_id, bucket_name FROM value_attributions
			WHERE partner_id IS NOT NULL 
				AND user_agent IS NULL
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

	rows, err := conn.Query(ctx, `
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
			WHERE value_attributions.project_id IN (SELECT unnest($3::bytea[]))
				AND value_attributions.bucket_name IN (SELECT unnest($4::bytea[]))
			RETURNING partner_id, user_agent
		)
		SELECT partner_id, user_agent
		FROM updated
		`, pgutil.UUIDArray(p.UUIDs), pgutil.ByteaArray(p.Names), pgutil.ByteaArray(projects), pgutil.ByteaArray(buckets),
	)
	if err != nil {
		return errs.New("updating rows: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var partnerID, userAgent pgtype.GenericBinary
		err = rows.Scan(&partnerID, &userAgent)
		if err != nil {
			return errs.New("scanning row: %w", err)
		}
		id, err := uuid.FromBytes(partnerID.Bytes)
		if err != nil {
			return errs.New("error getting partnerID from bytes: %w", err)
		}
		log.Info("migrated row", zap.String("partner_id", id.String()), zap.String("user_agent", string(userAgent.Bytes)))
	}
	return rows.Err()
}
