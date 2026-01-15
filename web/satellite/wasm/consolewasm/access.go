// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consolewasm

import (
	"encoding/base64"

	"storj.io/common/encryption"
	"storj.io/common/grant"
	"storj.io/common/macaroon"
	"storj.io/common/storj"
)

// GenAccessGrant creates a new access grant with optional path encryption
// and returns it serialized form.
func GenAccessGrant(satelliteNodeURL, apiKey, encryptionPassphrase, base64EncodedSalt string, encryptPath bool) (string, error) {
	parsedAPIKey, err := macaroon.ParseAPIKey(apiKey)
	if err != nil {
		return "", err
	}

	key, err := DeriveRootKey(encryptionPassphrase, base64EncodedSalt)
	if err != nil {
		return "", err
	}

	encAccess := grant.NewEncryptionAccessWithDefaultKey(key)
	encAccess.SetDefaultPathCipher(storj.EncAESGCM)
	if !encryptPath {
		encAccess.SetDefaultPathCipher(storj.EncNull)
	}
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
func DeriveRootKey(encryptionPassphrase, base64EncodedSalt string) (*storj.Key, error) {
	const concurrency = 8
	saltBytes, err := base64.StdEncoding.DecodeString(base64EncodedSalt)
	if err != nil {
		return nil, err
	}
	return encryption.DeriveRootKey([]byte(encryptionPassphrase), saltBytes, "", concurrency)
}
