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
	fmt.Println("encrypted path: ", hex.EncodeToString([]byte(encryptedPath.Raw())))

	// decrypting the path
	decryptedPath, err := encryption.DecryptPath("bucket", encryptedPath, storj.EncAESGCM, store)
	if err != nil {
		panic(err)
	}
	fmt.Println("decrypted path: ", decryptedPath)

	// Output:
	// root key (32 bytes): 000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f
	// path to encrypt: fold1/fold2/fold3/file.txt
	// encrypted path:  02387ce34e2054bcb9a0428b820102876eef8325a8397bf7568e91afc40739406ffad12f02453d291b9cb8947155462d6c1edc2367507b0de55b46fa7231f3ba6ad7ce79f4822f02ad7257e8ef4f938ac6b6794b50852873d1b3d32e018dfb17a674dc806ac6e8ddd4262f02aa2128dc8614940f7cf6628513b581f7c18724af3c01018f7c861520c2fdfd78f7b1b25ce0
	// decrypted path:  fold1/fold2/fold3/file.txt
}
