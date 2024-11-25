// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consolewasm"
	"storj.io/storj/satellite/migration"
)

// MigrateSatelliteDB migrates satellite database.
func MigrateSatelliteDB(ctx context.Context, log *zap.Logger, db satellite.DB, migrationType string) (err error) {
	for _, migrationType := range strings.Split(migrationType, ",") {
		switch migrationType {
		case migration.FullMigration:
			err = db.MigrateToLatest(ctx)
			if err != nil {
				return err
			}
		case migration.SnapshotMigration:
			log.Info("MigrationUnsafe using latest snapshot. It's not for production", zap.String("db", "master"))
			err = db.Testing().TestMigrateToLatest(ctx)
			if err != nil {
				return err
			}
		case migration.TestDataCreation:
			err := createTestData(ctx, db)
			if err != nil {
				return err
			}
		case migration.NoMigration:
		// noop
		default:
			return errs.New("unsupported migration type: %s, please try one of the: %s", migrationType, strings.Join(migration.MigrationTypes, ","))
		}
	}
	return err
}

var (
	projectID = uuid.UUID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
	apiKeyID  = uuid.UUID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 2})
	head      = []byte{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3,
	}
	secret = []byte{
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4,
	}
	password = "password"
)

// createTestData creates predefined test account to make the integration tests easier.
func createTestData(ctx context.Context, db satellite.DB) error {
	userID, err := uuid.FromString("be041c3c-0658-40d1-8f7c-e70a0a26cc12")
	if err != nil {
		return err
	}

	_, err = db.Console().Users().Get(ctx, userID)
	if errors.Is(err, sql.ErrNoRows) {

		hash, err := bcrypt.GenerateFromPassword([]byte(password), 0)
		if err != nil {
			return err
		}

		_, err = db.Console().Users().Insert(ctx, &console.User{
			ID:                    userID,
			FullName:              "Hiro Protagonist",
			Email:                 "test@storj.io",
			ProjectLimit:          5,
			ProjectStorageLimit:   (memory.GB * 150).Int64(),
			ProjectBandwidthLimit: (memory.GB * 150).Int64(),
			PasswordHash:          hash,
		})
		if err != nil {
			return err
		}

		active := console.Active
		err = db.Console().Users().Update(ctx, userID, console.UpdateUserRequest{
			Status: &active,
		})
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	_, err = db.Console().Projects().Get(ctx, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		_, err := db.Console().Projects().Insert(ctx, &console.Project{
			ID:      projectID,
			OwnerID: userID,
			Name:    "testproject",
		})
		if err != nil {
			return err
		}
		_, err = db.Console().ProjectMembers().Insert(ctx, userID, projectID, console.RoleAdmin)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	_, err = db.Console().APIKeys().GetByNameAndProjectID(ctx, "testkey", projectID)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = db.Console().APIKeys().Create(ctx, head, console.APIKeyInfo{
			ID:        apiKeyID,
			ProjectID: projectID,
			Name:      "testkey",
			Secret:    secret,
		})
	} else if err != nil {
		return err
	}
	return err
}

// GetTestApiKey can calculate an access grant for the predefined test users/project.
func GetTestApiKey(satelliteId string) (string, error) {
	key, err := macaroon.FromParts(head, secret)
	if err != nil {
		return "", errs.Wrap(err)
	}

	idHash := sha256.Sum256(projectID[:])
	base64Salt := base64.StdEncoding.EncodeToString(idHash[:])

	accessGrant, err := consolewasm.GenAccessGrant(satelliteId, key.Serialize(), password, base64Salt)
	if err != nil {
		return "", errs.Wrap(err)
	}

	return accessGrant, nil
}
