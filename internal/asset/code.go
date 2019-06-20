// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package asset

import (
	"bytes"
	"encoding/base64"
	"fmt"
)

// GenerateGo generates an function closure that can be assigned to a variable.
func (asset *Asset) GenerateGo() []byte {
	var source bytes.Buffer
	fmt.Fprintf(&source, "func() *Asset {\n")

	blob := []byte{}
	blobMapping := map[*Asset][2]int{}

	var writeBlob func(asset *Asset)
	writeBlob = func(asset *Asset) {
		if !asset.Mode.IsDir() {
			start := len(blob)
			blob = append(blob, asset.Data...)
			finish := len(blob)
			blobMapping[asset] = [2]int{start, finish}
			return
		}

		for _, child := range asset.Children {
			writeBlob(child)
		}
	}
	writeBlob(asset)

	blob64 := base64.StdEncoding.EncodeToString(blob)
	fmt.Fprintf(&source, "blob, err := base64.StdEncoding.DecodeString(%q)\n", blob64)
	fmt.Fprintf(&source, "if err != nil {\n")
	fmt.Fprintf(&source, "    panic(err)\n")
	fmt.Fprintf(&source, "}\n\n")

	var writeAsset func(asset *Asset)
	writeAsset = func(asset *Asset) {
		fmt.Fprintf(&source, "&Asset{\n")
		defer fmt.Fprintf(&source, "}")

		fmt.Fprintf(&source, "Name: %q,\n", asset.Name)
		fmt.Fprintf(&source, "Mode: %o,\n", asset.Mode)
		//TODO: fmt.Fprintf(&source, "ModTime: %v,\n", asset.ModTime)

		if !asset.Mode.IsDir() {
			r := blobMapping[asset]
			fmt.Fprintf(&source, "Data: blob[%d:%d],\n", r[0], r[1])
			return
		}

		fmt.Fprintf(&source, "Children: []*Asset{\n")
		for _, child := range asset.Children {
			writeAsset(child)
			fmt.Fprintf(&source, ",\n")
		}
		fmt.Fprintf(&source, "},\n")
	}

	fmt.Fprintf(&source, "return ")
	writeAsset(asset)
	fmt.Fprintf(&source, "\n")

	fmt.Fprintf(&source, "}()\n")
	return source.Bytes()
}
