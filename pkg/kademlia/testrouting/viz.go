// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testrouting

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
)

// Graph writes a DOT format visual graph description of the routing table to w
func (t *Table) Graph(w io.Writer) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	var buf bytes.Buffer
	buf.Write([]byte("digraph{node [shape=box];"))
	t.graph(&buf, t.makeTree())
	buf.Write([]byte("}\n"))

	_, err := buf.WriteTo(w)
	return err
}

func (t *Table) graph(buf *bytes.Buffer, b *bucket) {
	if t.splits[b.prefix] {
		fmt.Fprintf(buf, "b%s [label=%q];", b.prefix, b.prefix)
		if b.similar != nil {
			t.graph(buf, b.similar)
			t.graph(buf, b.dissimilar)
			fmt.Fprintf(buf, "b%s -> {b%s, b%s};",
				b.prefix, b.similar.prefix, b.dissimilar.prefix)
		}
		return
	}
	// b.prefix is only ever 0s or 1s, so we don't need escaping below.
	fmt.Fprintf(buf, "b%s [label=\"%s\nrouting:\\l", b.prefix, b.prefix)
	for _, node := range b.nodes {
		fmt.Fprintf(buf, "  %s\\l", hex.EncodeToString(node.node.Id[:]))
	}
	fmt.Fprintf(buf, "cache:\\l")
	for _, node := range b.cache {
		fmt.Fprintf(buf, "  %s\\l", hex.EncodeToString(node.node.Id[:]))
	}
	fmt.Fprintf(buf, "\"];")
}
