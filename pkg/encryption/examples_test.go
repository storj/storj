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
	fmt.Println("encrypted path: ", []byte(encryptedPath))

	// decrypting the path
	decryptedPath, err := encryption.DecryptPath(encryptedPath, storj.AESGCM, seed)
	if err != nil {
		panic(err)
	}
	fmt.Println("decrypted path: ", decryptedPath)

	// handling of shared path
	sharedPath := storj.JoinPaths(storj.SplitPath(encryptedPath)[2:]...)
	fmt.Println("shared path:    ", []byte(sharedPath))
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
	// encrypted path:  [186 188 110 99 58 134 253 153 73 124 24 100 25 172 252 237 107 239 156 38 90 96 62 234 127 95 217 55 243 221 247 89 249 47 35 41 220 15 2 225 88 251 248 23 177 26 161 72 175 192 128 167 81 226 76 90 85 39 48 4 203 225 246 154 160 125 172 47 255 88 34 181 126 174 61 221 222 181 205 209 130 128 253 71 92 26 79 147 15 42 185 107 99 125 158 135 63 239 170 83 175 47 234 163 184 13 78 64 28 86 155 19 106 251 134 96 26 186 249 232 154 251 77 7 43 142 248 80 176 225 200 127 197 165 81 221 35 196]
	// decrypted path:  fold1/fold2/fold3/file.txt
	// shared path:     [255 88 34 181 126 174 61 221 222 181 205 209 130 128 253 71 92 26 79 147 15 42 185 107 99 125 158 135 63 239 170 83 175 47 234 163 184 13 78 64 28 86 155 19 106 251 134 96 26 186 249 232 154 251 77 7 43 142 248 80 176 225 200 127 197 165 81 221 35 196]
	// derived key (32 bytes): 909db5ccf2b645e3352ee8212305596ed514d9f84d5acd21d93b4527d2a0c7e1
	// decrypted path:  fold3/file.txt
}
