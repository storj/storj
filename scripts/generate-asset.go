// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"log"

	"storj.io/storj/internal/asset"
)

func main() {
	packageName := flag.String("pkg", "", "package name")
	variableName := flag.String("var", "", "variable name to assign to")
	dir := flag.String("dir", "", "directory")

	flag.Parse()

	asset, err := asset.NewDir(*dir)
	if err != nil {
		log.Fatal(err)
	}

	var code bytes.Buffer
	fmt.Fprintf(&code, "package %s\n\n", *packageName)

	fmt.Fprintf(&code, "import (\n")
	fmt.Fprintf(&code, "\t\t\"encoding/base64\"\n")
	fmt.Fprintf(&code, ")\n\n")

	fmt.Fprintf(&code, "func init() {\n")
	fmt.Fprintf(&code, "%s = ", *variableName)
	code.Write(asset.GenerateGo())
	fmt.Fprintf(&code, "}\n")

	formatted, err := format.Source(code.Bytes())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(formatted))
}
