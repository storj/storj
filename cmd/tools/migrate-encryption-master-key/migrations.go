// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/kms"
	"storj.io/storj/shared/dbutil/pgutil"
	"storj.io/storj/shared/dbutil/retrydb"
)

// MigrateEncryptionPassphrases updates the encryption key ID and encrypted passphrase of projects from a specified
// encryption master key to another.
func MigrateEncryptionPassphrases(ctx context.Context, log *zap.Logger, conn *pgx.Conn, config Config) (err error) {
	defer mon.Task()(&ctx)(&err)

	lastID := make([]byte, 0)
	var total int
	var rowsFound bool

	kmsService := kms.NewService(kms.Config{
		Provider:         config.Provider,
		KeyInfos:         config.AllKeyInfos(),
		DefaultMasterKey: config.NewKeyID(),
		MockClient:       config.TestMockKmsClient,
	})
	err = kmsService.Initialize(ctx)

	if err != nil {
		return errs.New("error initializing kms service: %w", err)
	}

	for {
		rowsFound = false
		idsToUpdate := struct {
			ids              [][]byte
			newEncPassphrase [][]byte
		}{}
		err = func() error {
			rows, err := conn.Query(ctx, `
				SELECT id, passphrase_enc FROM projects
				WHERE id > $1
				 AND passphrase_enc_key_id = $2
				ORDER BY id
				LIMIT $3
			`, lastID, config.OldKeyID(), config.Limit)
			if err != nil {
				return errs.New("error select ids for update: %w", err)
			}
			defer rows.Close()

			for rows.Next() {
				rowsFound = true

				var id, passphraseEnc []byte
				err = rows.Scan(&id, &passphraseEnc)
				if err != nil {
					return errs.New("error scanning results from select: %w", err)
				}

				plainPassphrase, err := kmsService.DecryptPassphrase(ctx, config.OldKeyID(), passphraseEnc)
				if err != nil {
					return errs.New("error decrypting passphrase: %w", err)
				}
				newPassphraseEnc, _, err := kmsService.EncryptPassphrase(ctx, plainPassphrase)
				if err != nil {
					return errs.New("error encrypting passphrase: %w", err)
				}

				idsToUpdate.ids = append(idsToUpdate.ids, id)
				idsToUpdate.newEncPassphrase = append(idsToUpdate.newEncPassphrase, newPassphraseEnc)
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
						unnest($2::bytea[]) as new_enc_passphrase
				),
				updated as (
					UPDATE projects
					SET passphrase_enc_key_id = $3,
						passphrase_enc = to_update.new_enc_passphrase
					FROM to_update
					WHERE projects.id = to_update.id
					RETURNING 1
				)
				SELECT count(*)
				FROM updated;
			`, pgutil.ByteaArray(idsToUpdate.ids), pgutil.ByteaArray(idsToUpdate.newEncPassphrase), config.NewKeyID(),
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
	log.Info("projects encryption passphrase migration complete", zap.Int("total rows updated", total))
	return nil
}
