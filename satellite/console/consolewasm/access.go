// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consolewasm

import (
	"crypto/sha256"

	"storj.io/common/encryption"
	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/uplink/private/access2"
)

// GenAccessGrant creates a new access grant and returns it serialized form.
func GenAccessGrant(satelliteNodeURL, apiKey, encryptionPassphrase, projectID string) (string, error) {
	parsedAPIKey, err := macaroon.ParseAPIKey(apiKey)
	if err != nil {
		return "", err
	}

	id, err := uuid.FromString(projectID)
	if err != nil {
		return "", err
	}

	const concurrency = 8
	salt := sha256.Sum256(id[:])

	key, err := encryption.DeriveRootKey([]byte(encryptionPassphrase), salt[:], "", concurrency)
	if err != nil {
		return "", err
	}

	encAccess := access2.NewEncryptionAccessWithDefaultKey(key)
	encAccess.SetDefaultPathCipher(storj.EncAESGCM)
	a := &access2.Access{
		SatelliteAddress: satelliteNodeURL,
		APIKey:           parsedAPIKey,
		EncAccess:        encAccess,
	}
	accessString, err := a.Serialize()
	if err != nil {
		return "", err
	}
	return accessString, nil
}
