package main

import (
	"context"
	"fmt"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

var mon = monkit.Package()

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	defer mon.Task()(&ctx)(nil)
	defer mon.Task()(&ctx)(nil)
	defer mon.Task()(&ctx)(nil)
	defer mon.Task()(&ctx)(nil)
	defer mon.Task()(&ctx)(nil)
	defer mon.Task()(&ctx)(nil)
	defer mon.Task()(&ctx)(nil)
	defer mon.Task()(&ctx)(nil)
	defer mon.Task()(&ctx)(nil)
	defer mon.Task()(&ctx)(nil)
	defer mon.Task()(&ctx)(nil)
	defer mon.Task()(&ctx)(nil)
	defer mon.Task()(&ctx)(nil)
	defer mon.Task()(&ctx)(nil)

	cancel()
	select {
	case <-ctx.Done():
		fmt.Println("done", ctx.Err())
	default:
		fmt.Println("not done")
	}
}
