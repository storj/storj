// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"flag"
	"os"
	"time"
)

func main() {
	host := flag.String("host_port", "", "host and port")
	flag.Parse()

	if *host != "127.0.0.1:0" {
		panic("incorrect host port")
	}

	_, _ = os.Stderr.WriteString("WARNING: All log messages before absl::InitializeLog() is called are written to STDERR\n")
	time.Sleep(30 * time.Millisecond)
	_, _ = os.Stderr.WriteString("I0000 00:00:1764151016.243879      50 emulator_main.cc:39] Cloud Spanner Emulator running.\n")
	time.Sleep(30 * time.Millisecond)

	_, _ = os.Stderr.WriteString("I0000 00:00:1764151016.245399      50 emulator_main.cc:40] Server address: 127.0.")
	time.Sleep(30 * time.Millisecond)
	_, _ = os.Stderr.WriteString("0.1:46061\n")
	time.Sleep(5 * time.Minute)
}
