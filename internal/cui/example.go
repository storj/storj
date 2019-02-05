// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/cui"
)

func main() {
	screen, err := cui.NewScreen()
	if err != nil {
		fmt.Println(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var group errgroup.Group
	group.Go(func() error {
		defer cancel()
		return screen.Run()
	})

	counter := 0
	for ctx.Err() == nil {
		width, _ := screen.Size()
		fmt.Fprintf(screen, "%2d\n", counter)
		fmt.Fprintf(screen, "%s\n", strings.Repeat("=", width))
		screen.Flush()

		counter++
		time.Sleep(time.Second)
	}
	if err := screen.Close(); err != nil {
		fmt.Println(err)
	}

	if err := group.Wait(); err != nil {
		fmt.Println(err)
	}
}
