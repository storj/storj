// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"

	"storj.io/common/base58"
	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/storj"
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

	scope := new(pb.Scope)

	data, version, err := base58.CheckDecode(serializedAccess)
	if err != nil || version != 0 {
		panic(errs.New("invalid scope format"))
	}

	if err := pb.Unmarshal(data, scope); err != nil {
		panic(errs.New("unable to unmarshal scope: %v", err))
	}

	nodeURL, err := storj.ParseNodeURL(scope.SatelliteAddr)
	if err != nil {
		panic(err)
	}

	if !nodeURL.ID.IsZero() {
		fmt.Println(serializedAccess)
		return
	}

	nodeURL = storj.NodeURL{
		ID:      satNodeID,
		Address: scope.SatelliteAddr,
	}

	scope.SatelliteAddr = nodeURL.String()

	newdata, err := pb.Marshal(scope)
	if err != nil {
		panic(errs.New("unable to marshal scope: %v", err))
	}

	serialized := base58.CheckEncode(newdata, 0)
	fmt.Println(serialized)
}
