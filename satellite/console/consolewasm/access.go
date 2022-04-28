// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consolewasm

import (
	"crypto/sha256"

	"storj.io/common/encryption"
	"storj.io/common/grant"
	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// GenAccessGrant creates a new access grant and returns it serialized form.
func GenAccessGrant(satelliteNodeURL, apiKey, encryptionPassphrase, projectID string) (string, error) {
	parsedAPIKey, err := macaroon.ParseAPIKey(apiKey)
	if err != nil {
		return "", err
	}

	key, err := DeriveRootKey(encryptionPassphrase, projectID)
	if err != nil {
		return "", err
	}

	encAccess := grant.NewEncryptionAccessWithDefaultKey(key)
	encAccess.SetDefaultPathCipher(storj.EncAESGCM)
	encAccess.LimitTo(parsedAPIKey)

	accessString, err := (&grant.Access{
		SatelliteAddress: satelliteNodeURL,
		APIKey:           parsedAPIKey,
		EncAccess:        encAccess,
	}).Serialize()
	if err != nil {
		return "", err
	}
	return accessString, nil
}

// DeriveRootKey derives the root key portion of the access grant.
func DeriveRootKey(encryptionPassphrase, projectID string) (*storj.Key, error) {
	id, err := uuid.FromString(projectID)
	if err != nil {
		return nil, err
	}

	const concurrency = 8
	salt := sha256.Sum256(id[:])

	return encryption.DeriveRootKey([]byte(encryptionPassphrase), salt[:], "", concurrency)
}
