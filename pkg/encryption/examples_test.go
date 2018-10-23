// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption_test

import (
	"encoding/hex"
	"fmt"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/storj"
)

func ExampleEncryptPath() {
	var path = "fold1/fold2/fold3/file.txt"

	// seed
	seed := new(storj.Key)
	for i := range seed {
		seed[i] = byte(i)
	}
	fmt.Printf("root key (%d bytes): %s\n", len(seed), hex.EncodeToString(seed[:]))

	// use the seed for encrypting the path
	encryptedPath, err := encryption.EncryptPath(path, seed)
	if err != nil {
		panic(err)
	}
	fmt.Println("path to encrypt:", path)
	fmt.Println("encrypted path: ", encryptedPath)

	// decrypting the path
	decryptedPath, err := encryption.DecryptPath(encryptedPath, seed)
	if err != nil {
		panic(err)
	}
	fmt.Println("decrypted path: ", decryptedPath)

	// handling of shared path
	sharedPath := storj.TrimLeftPathComponents(encryptedPath, 2)
	fmt.Println("shared path:    ", sharedPath)
	derivedKey, err := encryption.DerivePathKey(decryptedPath, seed, 2)
	if err != nil {
		panic(err)
	}

	fmt.Printf("derived key (%d bytes): %s\n", len(derivedKey), hex.EncodeToString(derivedKey[:]))
	decryptedPath, err = encryption.DecryptPath(sharedPath, derivedKey)
	if err != nil {
		panic(err)
	}
	fmt.Println("decrypted path: ", decryptedPath)

	// Output:
	// root key (32 bytes): 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f
	// path to encrypt: fold1/fold2/fold3/file.txt
	// encrypted path:  1Gurj5g6pHBdX2RpO35yvBazY3OVr/19-Zy--UYZzsavcAPlpggCuVL_bmU/1iqr60cuSjXVINP994lMcUi60IzFr/1wFrLU7nBiaoreIe4wLR9NoCSV837t5H_
	// decrypted path:  fold1/fold2/fold3/file.txt
	// shared path:     1iqr60cuSjXVINP994lMcUi60IzFr/1wFrLU7nBiaoreIe4wLR9NoCSV837t5H_
	// derived key (32 bytes): 91b36388525a71160157741e1f3f696417f5305f0569b6eb5379b9b8dadaeb49
	// decrypted path:  fold3/file.txt
}
