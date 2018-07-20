// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/tyler-smith/go-bip39"

	"storj.io/storj/pkg/paths"
)

const mnemonic = "style inspire blade just ignore expose midnight maze " +
	"boring code burst host giraffe face parent basic ritual distance " +
	"trophy join relief hidden fine yard"

var path = paths.New("fold1/fold2/fold3/file.txt")

func main() {
	err := Main()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Main is the exported CLI executable function
func Main() error {
	fmt.Println("mnemonic:", mnemonic)
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		return err
	}
	fmt.Printf("root key (%d bytes): %s\n", len(seed),
		hex.EncodeToString(seed))
	encryptedPath, err := path.Encrypt(seed)
	if err != nil {
		return err
	}
	fmt.Println("path to encrypt:", path)
	fmt.Println("encrypted path: ", encryptedPath)
	decryptedPath, err := encryptedPath.Decrypt(seed)
	if err != nil {
		return err
	}
	fmt.Println("decrypted path: ", decryptedPath)
	sharedPath := encryptedPath[2:]
	fmt.Println("shared path:    ", encryptedPath[2:])
	derivedKey, err := decryptedPath.DeriveKey(seed, 2)
	if err != nil {
		return err
	}
	fmt.Printf("derived key (%d bytes): %s\n", len(derivedKey),
		hex.EncodeToString(derivedKey))
	decryptedPath, err = sharedPath.Decrypt(derivedKey)
	if err != nil {
		return err
	}
	fmt.Println("decrypted path: ", decryptedPath)

	// implement Bytes() function
	var pathBytes = path.Bytes()
	fmt.Println("path in Bytes is: ", pathBytes)
	return nil
}
