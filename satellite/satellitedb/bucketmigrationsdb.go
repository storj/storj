// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"database/sql"
	"errors"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/bucketmigrations"
	"storj.io/storj/satellite/satellitedb/dbx"
)

type bucketMigrationsDB struct {
	db *satelliteDB
}

// Create creates a new migration record.
func (db *bucketMigrationsDB) Create(ctx context.Context, migration bucketmigrations.Migration) (_ bucketmigrations.Migration, err error) {
	defer mon.Task()(&ctx)(&err)

	row, err := db.db.Create_BucketMigration(ctx,
		dbx.BucketMigration_Id(migration.ID[:]),
		dbx.BucketMigration_ProjectId(migration.ProjectID[:]),
		dbx.BucketMigration_BucketName([]byte(migration.BucketName)),
		dbx.BucketMigration_FromPlacement(migration.FromPlacement),
		dbx.BucketMigration_ToPlacement(migration.ToPlacement),
		dbx.BucketMigration_MigrationType(int(migration.MigrationType)),
		dbx.BucketMigration_State(string(migration.State)),
		dbx.BucketMigration_Create_Fields{},
	)
	if err != nil {
		return bucketmigrations.Migration{}, bucketmigrations.Error.Wrap(err)
	}

	return convertDBXToBucketMigration(row)
}

// Get retrieves a migration by ID.
func (db *bucketMigrationsDB) Get(ctx context.Context, id uuid.UUID) (_ bucketmigrations.Migration, err error) {
	defer mon.Task()(&ctx)(&err)

	row, err := db.db.Get_BucketMigration_By_Id(ctx, dbx.BucketMigration_Id(id[:]))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bucketmigrations.Migration{}, bucketmigrations.ErrMigrationNotFound.New("%s", id)
		}
		return bucketmigrations.Migration{}, bucketmigrations.Error.Wrap(err)
	}

	return convertDBXToBucketMigration(row)
}

// Update updates a migration's fields.
func (db *bucketMigrationsDB) Update(ctx context.Context, id uuid.UUID, update bucketmigrations.UpdateFields) (err error) {
	defer mon.Task()(&ctx)(&err)

	updateFields := dbx.BucketMigration_Update_Fields{}

	if update.State != nil {
		updateFields.State = dbx.BucketMigration_State(string(*update.State))
	}
	if update.BytesProcessed != nil {
		updateFields.BytesProcessed = dbx.BucketMigration_BytesProcessed(*update.BytesProcessed)
	}
	if update.ErrorMessage != nil {
		updateFields.ErrorMessage = dbx.BucketMigration_ErrorMessage(*update.ErrorMessage)
	}
	if update.CompletedAt != nil {
		updateFields.CompletedAt = dbx.BucketMigration_CompletedAt(*update.CompletedAt)
	}

	_, err = db.db.Update_BucketMigration_By_Id(
		ctx,
		dbx.BucketMigration_Id(id[:]),
		updateFields,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return bucketmigrations.ErrMigrationNotFound.New("%s", id)
		}
		return bucketmigrations.Error.Wrap(err)
	}

	return nil
}

// Delete removes a migration record.
func (db *bucketMigrationsDB) Delete(ctx context.Context, id uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	deleted, err := db.db.Delete_BucketMigration_By_Id(ctx, dbx.BucketMigration_Id(id[:]))
	if err != nil {
		return bucketmigrations.Error.Wrap(err)
	}
	if !deleted {
		return bucketmigrations.ErrMigrationNotFound.New("%s", id)
	}

	return nil
}

// ListByBucket returns all migrations for a specific bucket, ordered by created_at descending.
func (db *bucketMigrationsDB) ListByBucket(ctx context.Context, projectID uuid.UUID, bucketName string) (migrations []bucketmigrations.Migration, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.db.All_BucketMigration_By_ProjectId_And_BucketName_OrderBy_Desc_CreatedAt(
		ctx,
		dbx.BucketMigration_ProjectId(projectID[:]),
		dbx.BucketMigration_BucketName([]byte(bucketName)),
	)
	if err != nil {
		return nil, bucketmigrations.Error.Wrap(err)
	}

	migrations = make([]bucketmigrations.Migration, 0, len(rows))
	for _, row := range rows {
		migration, err := convertDBXToBucketMigration(row)
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, migration)
	}

	return migrations, nil
}

// ListByState returns migrations in a specific state with pagination.
func (db *bucketMigrationsDB) ListByState(ctx context.Context, opts bucketmigrations.ListOptions) (migrations []bucketmigrations.Migration, err error) {
	defer mon.Task()(&ctx)(&err)

	rows, err := db.db.Limited_BucketMigration_By_State_OrderBy_Asc_CreatedAt(
		ctx,
		dbx.BucketMigration_State(string(opts.State)),
		opts.Limit,
		int64(opts.Offset),
	)
	if err != nil {
		return nil, bucketmigrations.Error.Wrap(err)
	}

	migrations = make([]bucketmigrations.Migration, 0, len(rows))
	for _, row := range rows {
		migration, err := convertDBXToBucketMigration(row)
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, migration)
	}

	return migrations, nil
}

// convertDBXToBucketMigration converts a DBX row to a Migration struct.
func convertDBXToBucketMigration(row *dbx.BucketMigration) (bucketmigrations.Migration, error) {
	id, err := uuid.FromBytes(row.Id)
	if err != nil {
		return bucketmigrations.Migration{}, bucketmigrations.Error.Wrap(err)
	}

	projectID, err := uuid.FromBytes(row.ProjectId)
	if err != nil {
		return bucketmigrations.Migration{}, bucketmigrations.Error.Wrap(err)
	}

	migration := bucketmigrations.Migration{
		ID:             id,
		ProjectID:      projectID,
		BucketName:     string(row.BucketName),
		FromPlacement:  row.FromPlacement,
		ToPlacement:    row.ToPlacement,
		MigrationType:  bucketmigrations.MigrationType(row.MigrationType),
		State:          bucketmigrations.State(row.State),
		BytesProcessed: row.BytesProcessed,
		ErrorMessage:   row.ErrorMessage,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
		CompletedAt:    row.CompletedAt,
	}

	return migration, nil
}
