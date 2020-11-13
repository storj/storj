// Copyright (C) 2022 Storj, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"storj.io/common/storj"
)

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s <nodeid>\n", os.Args[0])
	os.Exit(1)
}

func output(id storj.NodeID) {
	fmt.Printf("base58 id: %s\n", id.String())
	fmt.Printf("hex id: %x\n", id.Bytes())
	fmt.Printf("version: %d\n", id.Version().Number)
	diff, err := id.Difficulty()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting difficulty: %v\n", err)
	} else {
		fmt.Printf("difficulty: %d\n", diff)
	}
}

func main() {
	if len(os.Args) != 2 {
		usage()
	}

	id, err := storj.NodeIDFromString(os.Args[1])
	if err == nil {
		output(id)
		return
	}

	idBytes, err := hex.DecodeString(os.Args[1])
	if err == nil {
		id, err := storj.NodeIDFromBytes(idBytes)
		if err == nil {
			output(id)
			return
		}
	}

	fmt.Fprintf(os.Stderr, "unknown argument: %q", os.Args[1])
	usage()
}
