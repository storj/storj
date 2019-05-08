// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"errors"

	"github.com/skyrings/skyring-common/tools/uuid"
	"storj.io/storj/satellite/console"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type resetPasswordTokens struct {
	db dbx.Methods
}

func (rpt *resetPasswordTokens) Create(ctx context.Context, ownerID *uuid.UUID) (*console.ResetPasswordToken, error) {
	secret, err := console.NewResetPasswordSecret()
	if err != nil {
		return nil, err
	}

	resToken, err := rpt.db.Create_ResetPasswordToken(
		ctx,
		dbx.ResetPasswordToken_Secret(secret[:]),
		dbx.ResetPasswordToken_OwnerId(ownerID[:]),
	)
	if err != nil {
		return nil, err
	}

	return resetPasswordTokenFromDBX(resToken)
}

func (rpt *resetPasswordTokens) GetBySecret(ctx context.Context, secret console.ResetPasswordSecret) (*console.ResetPasswordToken, error) {
	resToken, err := rpt.db.Get_ResetPasswordToken_By_Secret(
		ctx,
		dbx.ResetPasswordToken_Secret(secret[:]),
	)
	if err != nil {
		return nil, err
	}

	return resetPasswordTokenFromDBX(resToken)
}

func (rpt *resetPasswordTokens) GetByOwnerID(ctx context.Context, ownerID uuid.UUID) (*console.ResetPasswordToken, error) {
	resToken, err := rpt.db.Get_ResetPasswordToken_By_OwnerId(
		ctx,
		dbx.ResetPasswordToken_OwnerId(ownerID[:]),
	)
	if err != nil {
		return nil, err
	}

	return resetPasswordTokenFromDBX(resToken)
}

func (rpt *resetPasswordTokens) Delete(ctx context.Context, secret console.ResetPasswordSecret) error {
	_, err := rpt.db.Delete_ResetPasswordToken_By_Secret(
		ctx,
		dbx.ResetPasswordToken_Secret(secret[:]),
	)

	return err
}

func resetPasswordTokenFromDBX(resetToken *dbx.ResetPasswordToken) (*console.ResetPasswordToken, error) {
	if resetToken == nil {
		return nil, errors.New("token parameter is nil")
	}

	var secret [32]byte

	copy(secret[:], resetToken.Secret)

	result := &console.ResetPasswordToken{
		Secret:    secret,
		OwnerId:   nil,
		CreatedAt: resetToken.CreatedAt,
	}

	if resetToken.OwnerId != nil {
		ownerID, err := bytesToUUID(resetToken.OwnerId)
		if err != nil {
			return nil, err
		}

		result.OwnerId = &ownerID
	}

	return result, nil
}
