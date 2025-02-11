// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

func summarizeVerificationLog(cmd *cobra.Command, args []string) error {
	f, err := os.Open(args[0])
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = f.Close() }()

	count := map[storj.NodeID]int{}

	s := bufio.NewScanner(f)
	for s.Scan() {
		line := s.Text()

		const head = "Node ID: "
		p := strings.Index(line, head)
		if p < 0 {
			continue
		}
		line = line[p+len(head):]

		end := strings.Index(line, ",")
		if end < 0 {
			return fmt.Errorf("invalid line %q", line)
		}

		id, err := storj.NodeIDFromString(line[:end])
		if err != nil {
			return errs.Wrap(err)
		}

		count[id]++
	}
	if s.Err() != nil {
		return errs.Wrap(s.Err())
	}

	type Pair struct {
		Key   storj.NodeID
		Value int
	}
	var pairs []Pair

	for id, count := range count {
		pairs = append(pairs, Pair{Key: id, Value: count})
	}

	sort.Slice(pairs, func(i, k int) bool {
		return pairs[i].Value < pairs[k].Value
	})

	fmt.Println("node id, line count")
	for _, p := range pairs {
		fmt.Print(p.Key, ", ", p.Value, "\n")
	}

	return nil
}
