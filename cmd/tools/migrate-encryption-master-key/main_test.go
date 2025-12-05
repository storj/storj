// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	migrator "storj.io/storj/cmd/tools/migrate-encryption-master-key"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/kms"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/dbutil/tempdb"
)

func TestMigrateEncryptionPassphrases(t *testing.T) {
	t.Parallel()
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	config := migrator.Config{
		Provider: "gsm",
		Limit:    2,
		OldKeyInfo: kms.KeyInfos{
			Values: map[int]kms.KeyInfo{
				1: {SecretVersion: "some-old-key"},
			},
		},
		NewKeyInfo: kms.KeyInfos{
			Values: map[int]kms.KeyInfo{
				2: {SecretVersion: "some-new-key"},
			},
		},
		TestMockKmsClient: true,
	}
	oldKeyKmsService := kms.NewService(kms.Config{
		Provider:         config.Provider,
		KeyInfos:         config.AllKeyInfos(),
		DefaultMasterKey: config.OldKeyID(),
		MockClient:       config.TestMockKmsClient,
	})
	err := oldKeyKmsService.Initialize(ctx)
	require.NoError(t, err)

	newKeyKmsService := kms.NewService(kms.Config{
		Provider:         config.Provider,
		KeyInfos:         config.AllKeyInfos(),
		DefaultMasterKey: config.NewKeyID(),
		MockClient:       config.TestMockKmsClient,
	})
	err = newKeyKmsService.Initialize(ctx)
	require.NoError(t, err)

	for _, satelliteDB := range satellitedbtest.Databases(t) {
		t.Run(satelliteDB.Name, func(t *testing.T) {
			if satelliteDB.Name == "Spanner" {
				t.Skip("not implemented for spanner")
			}

			schemaSuffix := satellitedbtest.SchemaSuffix()
			schema := satellitedbtest.SchemaName(t.Name(), "category", 0, schemaSuffix)

			tempDB, err := tempdb.OpenUnique(ctx, log, satelliteDB.MasterDB.URL, schema, satelliteDB.MasterDB.ExtraStatements)
			require.NoError(t, err)

			db, err := satellitedbtest.CreateMasterDBOnTopOf(ctx, log, tempDB, satellitedb.Options{
				ApplicationName: "migrate-public-ids",
			})
			require.NoError(t, err)
			defer ctx.Check(db.Close)

			err = db.Testing().TestMigrateToLatest(ctx)
			require.NoError(t, err)

			mConnStr := strings.Replace(tempDB.ConnStr, "cockroach", "postgres", 1)

			conn, err := pgx.Connect(ctx, mConnStr)
			require.NoError(t, err)

			compromisedCount := 5
			safeCount := 5
			compromisedProjects := make(map[string]console.Project)
			safeProjects := make(map[string]console.Project)
			for i := 0; i < compromisedCount; i++ {
				encPassphrase, keyID, err := oldKeyKmsService.GenerateEncryptedPassphrase(ctx)
				require.NoError(t, err)

				p, err := db.Console().Projects().Insert(ctx, &console.Project{
					Name:               fmt.Sprintf("test%d", i),
					Description:        fmt.Sprintf("test%d", i),
					OwnerID:            testrand.UUID(),
					PassphraseEnc:      encPassphrase,
					PassphraseEncKeyID: &keyID,
				})
				require.NoError(t, err)
				compromisedProjects[p.ID.String()] = *p
			}
			for i := 0; i < safeCount; i++ {
				encPassphrase, keyID, err := newKeyKmsService.GenerateEncryptedPassphrase(ctx)
				require.NoError(t, err)

				p, err := db.Console().Projects().Insert(ctx, &console.Project{
					Name:               fmt.Sprintf("test%d", i),
					Description:        fmt.Sprintf("test%d", i),
					OwnerID:            testrand.UUID(),
					PassphraseEnc:      encPassphrase,
					PassphraseEncKeyID: &keyID,
				})
				require.NoError(t, err)
				safeProjects[p.ID.String()] = *p
			}

			_, err = db.Console().Projects().Insert(ctx, &console.Project{
				Name:        "test",
				Description: "test",
				OwnerID:     testrand.UUID(),
			})
			require.NoError(t, err)

			err = migrator.MigrateEncryptionPassphrases(ctx, log, conn, config)
			require.NoError(t, err)

			projects, err := db.Console().Projects().GetAll(ctx)
			require.NoError(t, err)

			updatedCount := 0
			unChangedCount := 0
			for _, project := range projects {
				if proj, ok := compromisedProjects[project.ID.String()]; ok {
					require.NotNil(t, project.PassphraseEnc)
					require.NotEqual(t, proj.PassphraseEnc, project.PassphraseEnc)
					require.NotEqual(t, proj.PassphraseEncKeyID, project.PassphraseEncKeyID)
					require.Equal(t, config.NewKeyID(), *project.PassphraseEncKeyID)

					oldPassphrase, err := oldKeyKmsService.DecryptPassphrase(ctx, *proj.PassphraseEncKeyID, proj.PassphraseEnc)
					require.NoError(t, err)
					newPassphrase, err := oldKeyKmsService.DecryptPassphrase(ctx, *project.PassphraseEncKeyID, project.PassphraseEnc)
					require.NoError(t, err)

					require.Equal(t, oldPassphrase, newPassphrase)

					updatedCount++
					continue
				}
				if proj, ok := safeProjects[project.ID.String()]; ok {
					require.NotNil(t, project.PassphraseEnc)
					require.Equal(t, proj.PassphraseEnc, project.PassphraseEnc)
					require.Equal(t, proj.PassphraseEncKeyID, project.PassphraseEncKeyID)
					require.Equal(t, config.NewKeyID(), *project.PassphraseEncKeyID)

					unChangedCount++
					continue
				}

				require.Nil(t, project.PassphraseEnc)
				require.Nil(t, project.PassphraseEnc)
				unChangedCount++
			}
			require.Equal(t, compromisedCount, updatedCount)
			require.Equal(t, safeCount+1, unChangedCount)
		})
	}
}
