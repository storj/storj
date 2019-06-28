// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"io/ioutil"
	"log"
	"os"

	"storj.io/storj/cmd/internal/asset"
)

func main() {
	packageName := flag.String("pkg", "", "package name")
	variableName := flag.String("var", "", "variable name to assign to")
	dir := flag.String("dir", "", "directory")
	out := flag.String("out", "", "output file")

	flag.Parse()

	asset, err := asset.ReadDir(*dir)
	if err != nil {
		log.Fatal(err)
	}

	var code bytes.Buffer
	fmt.Fprintf(&code, "// DO NOT COMMIT\n\n")
	fmt.Fprintf(&code, "package %s\n\n", *packageName)

	fmt.Fprintf(&code, "import (\n")
	fmt.Fprintf(&code, "\t\t\"storj.io/cmd/internal/asset\"\n")
	fmt.Fprintf(&code, ")\n\n")

	fmt.Fprintf(&code, "func init() {\n")
	fmt.Fprintf(&code, "%s = ", *variableName)
	code.Write(asset.InmemoryCode())
	fmt.Fprintf(&code, "}\n")

	formatted, err := format.Source(code.Bytes())
	if err != nil {
		fmt.Fprintln(os.Stderr, code.String())
		log.Fatal(err)
	}

	if *out == "" {
		fmt.Println(string(formatted))
	} else {
		err := ioutil.WriteFile(*out, formatted, 0644)
		if err != nil {
			log.Fatal(err)
		}
	}
}
