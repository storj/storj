// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package asset

import (
	"bytes"
	"fmt"
)

// InmemoryCode generates a function closure []byte that can be assigned to a variable.
func (asset *Asset) InmemoryCode() []byte {
	var source bytes.Buffer
	fmt.Fprintf(&source, "func() *asset.InmemoryFileSystem {\n")

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

	fmt.Fprintf(&source, "const blob = ")

	const lineLength = 120
	for len(blob) > 0 {
		if lineLength < len(blob) {
			fmt.Fprintf(&source, "\t%q +\n", string(blob[:lineLength]))
			blob = blob[lineLength:]
			continue
		}
		fmt.Fprintf(&source, "\t%q\n", string(blob))
		break
	}

	var writeAsset func(asset *Asset)
	writeAsset = func(asset *Asset) {
		fmt.Fprintf(&source, "{")
		defer fmt.Fprintf(&source, "}")
		if asset.Mode.IsDir() {
			fmt.Fprintf(&source, "\n")
		}
		fmt.Fprintf(&source, "Name: %q,", asset.Name)
		fmt.Fprintf(&source, "Mode: 0%o,", asset.Mode)
		fmt.Fprintf(&source, "ModTime: time.Unix(%d, 0),", asset.ModTime.Unix())

		if !asset.Mode.IsDir() {
			r := blobMapping[asset]
			fmt.Fprintf(&source, "Data: []byte(blob[%d:%d])", r[0], r[1])
			return
		}

		fmt.Fprintf(&source, "\nChildren: []*asset.Asset{\n")
		for _, child := range asset.Children {
			writeAsset(child)
			fmt.Fprintf(&source, ",\n")
		}
		fmt.Fprintf(&source, "},\n")
	}

	fmt.Fprintf(&source, "\n")
	fmt.Fprintf(&source, "return asset.Inmemory(&asset.Asset")
	writeAsset(asset)
	fmt.Fprintf(&source, ")\n")

	fmt.Fprintf(&source, "}()\n")
	return source.Bytes()
}
