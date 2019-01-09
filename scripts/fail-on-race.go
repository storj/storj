// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

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

	raceDetected := false
	search := []byte("DATA RACE")

	start := 0
	for {
		n, readErr := os.Stdin.Read(buffer[start:])
		end := start + n

		_, writeErr := os.Stdout.Write(buffer[start:end])
		if writeErr != nil {
			os.Stderr.Write([]byte(writeErr.Error()))
			os.Exit(2)
		}

		if bytes.Contains(buffer[:end], search) {
			raceDetected = true
			break
		}

		// copy buffer tail to the beginning of the content
		if end > len(search) {
			copy(buffer[:], buffer[end-len(search):end])
			start = len(search)
		}

		if readErr != nil {
			break
		}
	}

	_, _ = io.Copy(os.Stdout, os.Stdin)
	if raceDetected {
		os.Stderr.Write([]byte("\nTest failed due to data race.\n"))
		os.Exit(1)
	}
}
