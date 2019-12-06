// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	flag.Parse()
	if flag.Arg(0) == "" {
		fmt.Printf("usage: %s <targetdir>\n", os.Args[0])
		os.Exit(1)
	}
	err := Main()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// Main is the exported CLI executable function
func Main() error {
	pieces, err := ioutil.ReadDir(flag.Arg(0))
	if err != nil {
		return err
	}

	for _, piece := range pieces {
		pieceNum, err := strconv.Atoi(strings.TrimSuffix(piece.Name(), ".piece"))
		if err != nil {
			return err
		}
		pieceAddr := "localhost:" + strconv.Itoa(10000+pieceNum)
		piecePath := filepath.Join(flag.Arg(0), piece.Name())
		go fmt.Println(
			http.ListenAndServe(pieceAddr, http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					http.ServeFile(w, r, piecePath)
				})))
	}

	select {} // sleep forever
}
