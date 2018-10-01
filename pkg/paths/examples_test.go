// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package paths_test

import (
	"encoding/hex"
	"fmt"

	"github.com/tyler-smith/go-bip39"

	"storj.io/storj/pkg/paths"
)

func ExamplePath_Encrypt() {
	const mnemonic = "style inspire blade just ignore expose midnight maze " +
		"boring code burst host giraffe face parent basic ritual distance " +
		"trophy join relief hidden fine yard"

	var path = paths.New("fold1/fold2/fold3/file.txt")

	// create new seed from mnemonic
	fmt.Println("mnemonic:", mnemonic)
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		panic(err)
	}
	fmt.Printf("root key (%d bytes): %s\n", len(seed), hex.EncodeToString(seed))

	// use the seed for encrypting the path
	encryptedPath, err := path.Encrypt(seed)
	if err != nil {
		panic(err)
	}
	fmt.Println("path to encrypt:", path)
	fmt.Println("encrypted path: ", encryptedPath)

	// decrypting the path
	decryptedPath, err := encryptedPath.Decrypt(seed)
	if err != nil {
		panic(err)
	}
	fmt.Println("decrypted path: ", decryptedPath)

	// handling of shared path
	sharedPath := encryptedPath[2:]
	fmt.Println("shared path:    ", encryptedPath[2:])
	derivedKey, err := decryptedPath.DeriveKey(seed, 2)
	if err != nil {
		panic(err)
	}

	fmt.Printf("derived key (%d bytes): %s\n", len(derivedKey), hex.EncodeToString(derivedKey))
	decryptedPath, err = sharedPath.Decrypt(derivedKey)
	if err != nil {
		panic(err)
	}
	fmt.Println("decrypted path: ", decryptedPath)

	// implement Bytes() function
	var pathBytes = path.Bytes()
	fmt.Println("path in Bytes is: ", pathBytes)

	// Output:
	// mnemonic: style inspire blade just ignore expose midnight maze boring code burst host giraffe face parent basic ritual distance trophy join relief hidden fine yard
	// root key (64 bytes): 2cc5ebc6ea4c76be8e7347e7cefd0a36069ee14376fd5bc31350648e66999ae40765ea65e8239f9f63f90c7329d5a801b53592fd08016a426812adb625471ace
	// path to encrypt: fold1/fold2/fold3/file.txt
	// encrypted path:  1IxvMgcKDrllZs0fTSsCHTyB5LAoa/1lUAp4w2L7DKTPaW7ROooO19Al6Ai/1-7AAlcbkD5Gc6nCDq_Hb2sUNJEuL/1VNDpswHFmSY_3SU5_l3Ai5APPJ9aWIxR
	// decrypted path:  fold1/fold2/fold3/file.txt
	// shared path:     1-7AAlcbkD5Gc6nCDq_Hb2sUNJEuL/1VNDpswHFmSY_3SU5_l3Ai5APPJ9aWIxR
	// derived key (64 bytes): 367f581e49f0a501b64def5ec2a835d7193038c35a7562209c7015a1ee00e5c15e1aa41b5b7b5019ee906d5b76cee84b220fc6b900a55337a245071dbbc72c8f
	// decrypted path:  fold3/file.txt
	// path in Bytes is:  [102 111 108 100 49 47 102 111 108 100 50 47 102 111 108 100 51 47 102 105 108 101 46 116 120 116]
}
