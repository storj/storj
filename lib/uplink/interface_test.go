// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink_test

import (
	"fmt"

	"storj.io/storj/pkg/storj"
)

//This tests hello hello
func Example_interface() {
	const (
		myAPIKey = "change-me-to-the-api-key-created-in-satellite-gui"

		satellite       = "mars.tardigrade.io:7777"
		myBucket        = "my-first-bucket"
		myUploadPath    = "foo/bar/baz"
		myData          = "one fish two fish red fish blue fish"
		myEncryptionKey = "you'll never guess this"
	)

	var encryptionKey storj.Key
	copy(encryptionKey[:], []byte(myEncryptionKey))

	// apiKey, err := uplink.ParseAPIKey(myAPIKey)
	// if err != nil {
	// 	log.Fatalln("could not parse api key:", err)
	// }

	// err = WorkWithLibUplink(satellite, &encryptionKey, apiKey, myBucket, myUploadPath, []byte(myData))
	// if err != nil {
	// 	log.Fatalln("error:", err)
	// }

	fmt.Println("success!")

	// Output:
	// success!
}
