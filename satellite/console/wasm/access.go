// +build js,wasm
// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"syscall/js"

	"storj.io/common/encryption"
	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/uplink/private/access2"
)

func main() {
	js.Global().Set("generateAccessGrant", generateAccessGrant())
	<-make(chan bool)
}

func generateAccessGrant() js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) < 4 {
			return fmt.Sprintf("Error not enough arguments. Need 4, but only %d supplied. The order of arguments are: satellite Node URL, API key, encryption passphrase, and project salt.", len(args))
		}
		satelliteNodeURL := args[0].String()
		apiKey := args[1].String()
		encryptionPassphrase := args[2].String()
		projectSalt := args[3].String()

		return genAccessGrant(satelliteNodeURL,
			apiKey,
			encryptionPassphrase,
			projectSalt,
		)
	})
}

func genAccessGrant(satelliteNodeURL, apiKey, encryptionPassphrase, projectSalt string) string {
	parsedAPIKey, err := macaroon.ParseAPIKey(apiKey)
	if err != nil {
		return err.Error()
	}

	const concurrency = 8
	key, err := encryption.DeriveRootKey([]byte(encryptionPassphrase), []byte(projectSalt), "", concurrency)
	if err != nil {
		return err.Error()
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
		return err.Error()
	}
	return accessString
}
