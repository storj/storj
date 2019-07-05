// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption_test

import (
	"encoding/hex"
	"fmt"

	"storj.io/storj/pkg/encryption"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/storj"
)

func ExampleEncryptPath() {
	path := paths.NewUnencrypted("fold1/fold2/fold3/file.txt")

	// seed
	seed := new(storj.Key)
	for i := range seed {
		seed[i] = byte(i)
	}
	fmt.Printf("root key (%d bytes): %s\n", len(seed), hex.EncodeToString(seed[:]))

	store := encryption.NewStore()
	store.SetDefaultKey(seed)

	// use the seed for encrypting the path
	encryptedPath, err := encryption.EncryptPath("bucket", path, storj.EncAESGCM, store)
	if err != nil {
		panic(err)
	}
	fmt.Println("path to encrypt:", path)
	fmt.Println("encrypted path: ", encryptedPath)

	// decrypting the path
	decryptedPath, err := encryption.DecryptPath("bucket", encryptedPath, storj.EncAESGCM, store)
	if err != nil {
		panic(err)
	}
	fmt.Println("decrypted path: ", decryptedPath)

	// Output:
	// root key (32 bytes): 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f
	// path to encrypt: fold1/fold2/fold3/file.txt
	// encrypted path:  OHzjTiBUvLmgQouCAYdu74MlqDl791aOka_EBzlAb_rR/RT0pG5y4lHFVRi1sHtwjZ1B7DeVbRvpyMfO6atfOefSC/rXJX6O9Pk4rGtnlLUIUoc9Gz0y6N-xemdNyAasbo3dQm/qiEo3IYUlA989mKFE7WB98GHJK88AI98hhUgwv39ePexslzg
	// decrypted path:  fold1/fold2/fold3/file.txt
}
