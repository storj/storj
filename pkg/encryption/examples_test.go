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
	encryptedPath, err := encryption.EncryptPath(path, storj.AESGCM, seed)
	if err != nil {
		panic(err)
	}
	fmt.Println("path to encrypt:", path)
	fmt.Println("encrypted path: ", encryptedPath)

	// decrypting the path
	decryptedPath, err := encryption.DecryptPath(encryptedPath, storj.AESGCM, seed)
	if err != nil {
		panic(err)
	}
	fmt.Println("decrypted path: ", decryptedPath)

	// handling of shared path
	sharedPath := storj.JoinPaths(storj.SplitPath(encryptedPath)[2:]...)
	fmt.Println("shared path:    ", sharedPath)
	derivedKey, err := encryption.DerivePathKey(decryptedPath, seed, 2)
	if err != nil {
		panic(err)
	}

	fmt.Printf("derived key (%d bytes): %s\n", len(derivedKey), hex.EncodeToString(derivedKey[:]))
	decryptedPath, err = encryption.DecryptPath(sharedPath, storj.AESGCM, derivedKey)
	if err != nil {
		panic(err)
	}
	fmt.Println("decrypted path: ", decryptedPath)

	// Output:
	// root key (32 bytes): 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f
	// path to encrypt: fold1/fold2/fold3/file.txt
	// encrypted path:  urxuYzqG_ZlJfBhkGaz87WvvnCZaYD7qf1_ZN_Pd91n5/IyncDwLhWPv4F7EaoUivwICnUeJMWlUnMATL4faaoH2s/_1gitX6uPd3etc3RgoD9R1waT5MPKrlrY32ehz_vqlOv/6qO4DU5AHFabE2r7hmAauvnomvtNByuO-FCw4ch_xaVR3SPE
	// decrypted path:  fold1/fold2/fold3/file.txt
	// shared path:     _1gitX6uPd3etc3RgoD9R1waT5MPKrlrY32ehz_vqlOv/6qO4DU5AHFabE2r7hmAauvnomvtNByuO-FCw4ch_xaVR3SPE
	// derived key (32 bytes): 909db5ccf2b645e3352ee8212305596ed514d9f84d5acd21d93b4527d2a0c7e1
	// decrypted path:  fold3/file.txt
}
