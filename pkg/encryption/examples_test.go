// Copyright (C) 2019 Storj Labs, Inc.
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
	encryptedPath, err := encryption.EncryptPath(path, storj.EncAESGCM, seed)
	if err != nil {
		panic(err)
	}
	fmt.Println("path to encrypt:", path)
	fmt.Println("encrypted path: ", hex.EncodeToString([]byte(encryptedPath)))

	// decrypting the path
	decryptedPath, err := encryption.DecryptPath(encryptedPath, storj.EncAESGCM, seed)
	if err != nil {
		panic(err)
	}
	fmt.Println("decrypted path: ", decryptedPath)

	// handling of shared path
	sharedPath := storj.JoinPaths(storj.SplitPath(encryptedPath)[2:]...)
	fmt.Println("shared path:    ", hex.EncodeToString([]byte(sharedPath)))
	derivedKey, err := encryption.DerivePathKey(decryptedPath, seed, 2)
	if err != nil {
		panic(err)
	}

	fmt.Printf("derived key (%d bytes): %s\n", len(derivedKey), hex.EncodeToString(derivedKey[:]))
	decryptedPath, err = encryption.DecryptPath(sharedPath, storj.EncAESGCM, derivedKey)
	if err != nil {
		panic(err)
	}
	fmt.Println("decrypted path: ", decryptedPath)

	// Output:
	// root key (32 bytes): 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f
	// path to encrypt: fold1/fold2/fold3/file.txt
	// encrypted path:  02babc6e633a86fd99497c186419acfced6bef9c265a603eea7f5fd937f3ddf759f92f022329dc0f02e158fbf817b11aa148afc080a751e24c5a55273004cbe1f69aa07dac2f02fe025822b57eae3ddddeb5cdd18280fd475c1a4f930f2ab96b637d9e873fefaa53af2f02eaa3b80d4e401c569b136afb86601abaf9e89afb4d072b8ef850b0e1c87fc5a551dd23c4
	// decrypted path:  fold1/fold2/fold3/file.txt
	// shared path:     02fe025822b57eae3ddddeb5cdd18280fd475c1a4f930f2ab96b637d9e873fefaa53af2f02eaa3b80d4e401c569b136afb86601abaf9e89afb4d072b8ef850b0e1c87fc5a551dd23c4
	// derived key (32 bytes): 909db5ccf2b645e3352ee8212305596ed514d9f84d5acd21d93b4527d2a0c7e1
	// decrypted path:  fold3/file.txt
}
