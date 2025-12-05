// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

// metabase-minimize-listing-csv takes a CSV file as input and outputs a new CSV with
// equivalent order, but renaming each path component to minimize the input.
//
// Expected input is:
//
//	hex_object_key, version, status
package main

import (
	"bufio"
	"encoding/hex"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

func main() {
	flag.Parse()

	f := must(os.Open(flag.Arg(0)))
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	lines := 0

	var root node

	for scanner.Scan() {
		lines++

		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		tokens := strings.Split(scanner.Text(), ",")

		objectkey := string(must(hex.DecodeString(tokens[0])))
		version := must(strconv.Atoi(tokens[1]))
		status := must(strconv.Atoi(tokens[2]))

		root.insert(strings.Split(objectkey, "/"), state{version, status})
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	root.print("")
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

type state struct {
	version int
	status  int
}

type node struct {
	name   string
	states []state
	nodes  []node
}

func (n *node) insert(data []string, s state) {
	if len(data) == 0 {
		n.states = append(n.states, s)
		return
	}
	if len(n.nodes) == 0 || n.nodes[len(n.nodes)-1].name != data[0] {
		n.nodes = append(n.nodes, node{name: data[0]})
	}
	n.nodes[len(n.nodes)-1].insert(data[1:], s)
}

func (n *node) print(prefix string) {
	digits := int(math.Ceil(math.Log10(float64(len(n.nodes)))))
	format := "/%0" + strconv.Itoa(digits) + "d"
	for i := range n.nodes {
		c := n.nodes[i]
		c.print(prefix + fmt.Sprintf(format, i))
	}
	if len(n.nodes) == 0 {
		for _, s := range n.states {
			fmt.Printf("%v,%v,%v\n", prefix[1:], s.version, s.status)
		}
	}
}
