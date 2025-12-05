// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build ignore

package main

import (
	"flag"
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
	flag.Usage = func() {
		fmt.Println("usage: update-access [-a address] satellite-directory access")
		os.Exit(1)
	}

	address := flag.String("a", "", "satellite address")
	flag.Parse()

	args := flag.Args()

	if len(args) != 2 {
		flag.Usage()
	}

	satelliteDirectory := args[0]
	serializedAccess := args[1]

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

	if *address == "" && !nodeURL.ID.IsZero() {
		fmt.Println(serializedAccess)
		return
	}

	if *address == "" {
		address = &scope.SatelliteAddr
	}

	nodeURL = storj.NodeURL{
		ID:      satNodeID,
		Address: *address,
	}

	scope.SatelliteAddr = nodeURL.String()

	newdata, err := pb.Marshal(scope)
	if err != nil {
		panic(errs.New("unable to marshal scope: %v", err))
	}

	serialized := base58.CheckEncode(newdata, 0)
	fmt.Println(serialized)
}
