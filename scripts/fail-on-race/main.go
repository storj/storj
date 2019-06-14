// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"io"
	"os"
)

// fail-on-race detects for keyword "DATA RACE" in output
// and returns error code, if the output contains it.

func main() {
	var buffer [8192]byte

	problemDetected := false

	search := [][]byte{
		[]byte("DATA RACE"),
		[]byte("panic"),
	}

	maxsearch := 0
	for _, keyword := range search {
		if maxsearch < len(keyword) {
			maxsearch = len(keyword)
		}
	}

	start := 0
	for {
		n, readErr := os.Stdin.Read(buffer[start:])
		end := start + n

		_, writeErr := os.Stdout.Write(buffer[start:end])
		if writeErr != nil {
			os.Stderr.Write([]byte(writeErr.Error()))
			os.Exit(2)
		}

		for _, keyword := range search {
			if bytes.Contains(buffer[:end], keyword) {
				problemDetected = true
				break
			}
		}

		// copy buffer tail to the beginning of the content
		if end > maxsearch {
			copy(buffer[:], buffer[end-maxsearch:end])
			start = maxsearch
		}

		if readErr != nil {
			break
		}
	}

	_, _ = io.Copy(os.Stdout, os.Stdin)
	if problemDetected {
		os.Stderr.Write([]byte("\nTest failed due to data race or panic.\n"))
		os.Exit(1)
	}
}
