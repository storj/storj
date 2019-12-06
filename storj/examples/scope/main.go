// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"

	"storj.io/storj/lib/uplink"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage:", os.Args[0], "<scope>")
		os.Exit(1)
	}
	scope, err := uplink.ParseScope(os.Args[1])
	if err != nil {
		fmt.Println("invalid scope:", err.Error())
		os.Exit(1)
	}
	encaccess, err := scope.EncryptionAccess.Serialize()
	if err != nil {
		panic(err)
	}
	fmt.Printf("satellite: %s\napi key: %s\nenc access: %s\n",
		scope.SatelliteAddr,
		scope.APIKey.Serialize(),
		encaccess)
}
