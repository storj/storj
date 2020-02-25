// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"storj.io/common/identity"
	"storj.io/common/storj"
	"storj.io/storj/lib/uplink"
)

// This tool can be use to update existing access satellite address field to
// contain full node URL (NodeID + Satellite Address). As a result program
// will print out updated access.

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage: update-access satellite-directory access")
		os.Exit(1)
	}

	satelliteDirectory := os.Args[1]
	serializedAccess := os.Args[2]

	satNodeID, err := identity.NodeIDFromCertPath(filepath.Join(satelliteDirectory, "identity.cert"))
	if err != nil {
		panic(err)
	}

	access, err := uplink.ParseScope(serializedAccess)
	if err != nil {
		panic(err)
	}

	nodeURL, err := storj.ParseNodeURL(access.SatelliteAddr)
	if err != nil {
		panic(err)
	}

	if !nodeURL.ID.IsZero() {
		fmt.Println(serializedAccess)
		return
	}

	nodeURL = storj.NodeURL{
		ID:      satNodeID,
		Address: access.SatelliteAddr,
	}

	access.SatelliteAddr = nodeURL.String()

	serializedAccess, err = access.Serialize()
	if err != nil {
		panic(err)
	}
	fmt.Println(serializedAccess)
}
