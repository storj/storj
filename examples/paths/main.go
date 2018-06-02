// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/tyler-smith/go-bip39"

	"storj.io/storj/pkg/paths"
)

const mnemonic = "style inspire blade just ignore expose midnight maze " +
	"boring code burst host giraffe face parent basic ritual distance " +
	"trophy join relief hidden fine yard"

var path = []string{"fold1", "fold2", "fold3", "file.txt"}

func main() {
	err := Main()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Main is the exported CLI executable function
func Main() error {
	fmt.Printf("mnemonic: %s\n", mnemonic)
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		return err
	}
	fmt.Printf("root key (%d bytes): %s\n", len(seed),
		hex.EncodeToString(seed))
	encryptedPath, err := paths.Encrypt(path, seed)
	if err != nil {
		return err
	}
	fmt.Printf("path to encrypt: /%s\n", strings.Join(path, "/"))
	fmt.Printf("encrypted path:  /%s\n", strings.Join(encryptedPath, "/"))
	decryptedPath, err := paths.Decrypt(encryptedPath, seed)
	if err != nil {
		return err
	}
	fmt.Printf("decrypted path:  /%s\n", strings.Join(decryptedPath, "/"))
	sharedPath := encryptedPath[2:]
	fmt.Printf("shared path:     /%s\n", strings.Join(encryptedPath[2:], "/"))
	derivedKey := paths.DeriveKey(seed, path[:2])
	fmt.Printf("derived key (%d bytes): %s\n", len(derivedKey),
		hex.EncodeToString(derivedKey))
	decryptedPath, err = paths.Decrypt(sharedPath, derivedKey)
	if err != nil {
		return err
	}
	fmt.Printf("decrypted path:  /%s\n", strings.Join(decryptedPath, "/"))
	return nil
}
