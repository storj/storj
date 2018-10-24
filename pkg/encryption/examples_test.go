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
	sharedPath := storj.JoinPaths(storj.SplitPath(encryptedPath)[2:]...)
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
	// encrypted path:  -NFJg3s6dSsA9xnamPmiToejWCvyJop9IQrOc6kwSZV8/jBJ7q_fKGypIFF4I6QNSulyEi7lN3LqkG7PoeNW67PSQ/2hVJuIRjAQ57FTYPd4OCjrCbJit-L2hLLxQ9aD9QHGjG/D4CgV1J4Zv-Tfc4L0GmbuyqtQNJYUTNOtONlrTMgBxCv9ILh
	// decrypted path:  fold1/fold2/fold3/file.txt
	// shared path:     2hVJuIRjAQ57FTYPd4OCjrCbJit-L2hLLxQ9aD9QHGjG/D4CgV1J4Zv-Tfc4L0GmbuyqtQNJYUTNOtONlrTMgBxCv9ILh
	// derived key (32 bytes): c24f0fe828a5f67e230ec6a20d3bab3f2f31fab9f4af57ef654f152d2a51e31d
	// decrypted path:  fold3/file.txt
}
