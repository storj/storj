// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"storj.io/storj/storagenode/load"
)

func showIOStatsMain(args []string, output io.Writer) error {
	if len(args) < 2 || len(args) > 3 {
		return fmt.Errorf("usage: %s <pid> [<delay>]", args[0])
	}
	delay := time.Second
	var err error
	if len(args) == 3 {
		delay, err = time.ParseDuration(args[2])
		if err != nil {
			return fmt.Errorf("could not parse duration: %w", err)
		}
	}
	pid, err := strconv.Atoi(args[1])
	if err != nil {
		return err
	}
	if pid == 0 {
		pid = os.Getpid()
	}

	var stats1 load.Stats
	err = stats1.Get(int32(pid))
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(output, "%d: %+v\n", pid, stats1)
	time.Sleep(delay)

	var stats2 load.Stats
	err = stats2.Get(int32(pid))
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintf(output,
		"%d: %+v\n\n"+
			"Deltas:\n"+
			"ReadCount:  %d\n"+
			"WriteCount: %d\n"+
			"ReadBytes:  %d\n"+
			"WriteBytes: %d\n",
		pid,
		stats2,
		stats2.ReadCount-stats1.ReadCount,
		stats2.WriteCount-stats1.WriteCount,
		stats2.ReadBytes-stats1.ReadBytes,
		stats2.WriteBytes-stats1.WriteBytes)

	return nil
}

func main() {
	err := showIOStatsMain(os.Args, os.Stdout)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
