// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package paths_test

import (
	"encoding/hex"
	"fmt"

	"storj.io/storj/pkg/paths"
)

func ExamplePath_Encrypt() {
	var path = paths.New("fold1/fold2/fold3/file.txt")

	// seed
	seed := make([]byte, 64)
	for i := range seed {
		seed[i] = byte(i)
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
	// root key (64 bytes): 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f
	// path to encrypt: fold1/fold2/fold3/file.txt
	// encrypted path:  1ziIKg8Mw9_ywBqSOm78gQ9aQVbrQ/1FWdS4WUzuWXBrNML_FzGH8O2usos/1TKa8xYYCrUBEvEGj7YENdwViZOYh/17JWesXCBVU9Nx8IdnwuNUuwqgGFKL_jH
	// decrypted path:  fold1/fold2/fold3/file.txt
	// shared path:     1TKa8xYYCrUBEvEGj7YENdwViZOYh/17JWesXCBVU9Nx8IdnwuNUuwqgGFKL_jH
	// derived key (64 bytes): 2592f0694bc72a2719d09b7192b9b8f9e2517fda71419314d93a7c96844f28763ed3b829f3c9a09e812b402a66b1b0a832ae3cdd7435b7b496ca95eec108c629
	// decrypted path:  fold3/file.txt
	// path in Bytes is:  [102 111 108 100 49 47 102 111 108 100 50 47 102 111 108 100 51 47 102 105 108 101 46 116 120 116]
}
