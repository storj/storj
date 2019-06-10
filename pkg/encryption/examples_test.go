// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption_test

import (
	"encoding/hex"
	"fmt"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/storj"
)

func Example() {
	var bucket = "bucket"
	var bucketPath = storj.NewUnencryptedPath("fold1/fold2/fold3/file.txt").WithBucket(bucket)

	// Create a "random" key.
	var key storj.Key
	for i := range key {
		key[i] = byte(i)
	}
	fmt.Printf("root key (%d bytes): %s\n", len(key), hex.EncodeToString(key[:]))

	// Create a store and add some base keys.
	store := encryption.NewStore()
	store.Add(storj.NewUnencryptedPath("").WithBucket(bucket), storj.NewEncryptedPath(""), key)

	// Encrypt some path the store knows how to encrypt.
	encBucketPath, err := encryption.EncryptBucketPath(bucketPath, storj.AESGCM, store)
	if err != nil {
		panic(err)
	}
	fmt.Println("path to encrypt:", bucketPath)
	fmt.Println("encrypted path: ", encBucketPath)

	// Decrypt the same path.
	decBucketPath, err := encryption.DecryptBucketPath(encBucketPath, storj.AESGCM, store)
	if err != nil {
		panic(err)
	}
	fmt.Println("decrypted path: ", decBucketPath)

	// Output:
	// root key (32 bytes): 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f
	// path to encrypt: ubp:bucket/fold1/fold2/fold3/file.txt
	// encrypted path:  ebp:bucket/urxuYzqG_ZlJfBhkGaz87WvvnCZaYD7qf1_ZN_Pd91n5/IyncDwLhWPv4F7EaoUivwICnUeJMWlUnMATL4faaoH2s/_1gitX6uPd3etc3RgoD9R1waT5MPKrlrY32ehz_vqlOv/6qO4DU5AHFabE2r7hmAauvnomvtNByuO-FCw4ch_xaVR3SPE
	// decrypted path:  ubp:bucket/fold1/fold2/fold3/file.txt
}
