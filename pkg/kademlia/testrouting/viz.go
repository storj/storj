// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testrouting

import (
	"encoding/hex"
	"fmt"
	"io"
)

// Graph writes a DOT format visual graph description of the routing table to w
func (t *Table) Graph(w io.Writer) error {
	_, err := w.Write([]byte("digraph{node [shape=box];"))
	if err != nil {
		return err
	}
	err = t.graph(w, t.makeTree())
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("}\n"))
	return err
}

func (t *Table) graph(w io.Writer, b *bucket) error {
	if b.split {
		_, err := fmt.Fprintf(w, "b%s [label=\"%s\"];", b.prefix, b.prefix)
		if err != nil {
			return err
		}
		err = t.graph(w, b.similar)
		if err != nil {
			return err
		}
		err = t.graph(w, b.dissimilar)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "b%s -> {b%s, b%s};",
			b.prefix, b.similar.prefix, b.dissimilar.prefix)
		return err
	}
	_, err := fmt.Fprintf(w, "b%s [label=\"%s\nrouting:\\l", b.prefix, b.prefix)
	if err != nil {
		return err
	}
	for _, node := range b.nodes {
		_, err = fmt.Fprintf(w, "  %s\\l", hex.EncodeToString(node.node.Id[:]))
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "cache:\\l")
	if err != nil {
		return err
	}
	for _, node := range b.cache {
		_, err = fmt.Fprintf(w, "  %s\\l", hex.EncodeToString(node.node.Id[:]))
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(w, "\"];")
	return err
}
