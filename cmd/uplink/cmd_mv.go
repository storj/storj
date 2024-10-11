// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/common/sync2"
	"storj.io/storj/cmd/uplink/ulext"
	"storj.io/storj/cmd/uplink/ulfs"
	"storj.io/storj/cmd/uplink/ulloc"
)

type cmdMv struct {
	ex ulext.External

	access      string
	recursive   bool
	parallelism int
	dryrun      bool
	progress    bool

	source ulloc.Location
	dest   ulloc.Location
}

func newCmdMv(ex ulext.External) *cmdMv {
	return &cmdMv{ex: ex}
}

func (c *cmdMv) Setup(params clingy.Parameters) {
	c.access = params.Flag("access", "Access name or value to use", "").(string)
	c.recursive = params.Flag("recursive", "Move all objects or files under the specified prefix or directory", false,
		clingy.Short('r'),
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.parallelism = params.Flag("parallelism", "Controls how many objects will be moved in parallel", 1,
		clingy.Short('p'),
		clingy.Transform(strconv.Atoi),
		clingy.Transform(func(n int) (int, error) {
			if n <= 0 {
				return 0, errs.New("parallelism must be at least 1")
			}
			return n, nil
		}),
	).(int)
	c.dryrun = params.Flag("dryrun", "Print what operations would happen but don't execute them", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.progress = params.Flag("progress", "Show a progress bar when possible", true,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)

	c.source = params.Arg("source", "Source to move", clingy.Transform(ulloc.Parse)).(ulloc.Location)
	c.dest = params.Arg("dest", "Destination to move", clingy.Transform(ulloc.Parse)).(ulloc.Location)
}

func (c *cmdMv) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	fs, err := c.ex.OpenFilesystem(ctx, c.access)
	if err != nil {
		return err
	}
	defer func() { _ = fs.Close() }()

	switch {
	case c.source.Std() || c.dest.Std():
		return errs.New("cannot move to stdin/stdout")
	case c.source.String() == "" || c.dest.String() == "": // TODO maybe add Empty() method
		return errs.New("both source and dest cannot be empty")
	case (c.source.Local() && c.dest.Remote()) || (c.source.Remote() && c.dest.Local()):
		return errs.New("source and dest must be both local or both remote")
	case c.source.String() == c.dest.String():
		return errs.New("source and dest cannot be equal")
	case c.recursive && (!c.source.Directoryish() || !c.dest.Directoryish()):
		return errs.New("with --recursive flag source and destination must end with '/'")
	}

	// we ensure the source and destination are lexically directoryish
	// if they map to directories. the destination is always converted to be
	// directoryish if the copy is recursive.
	if fs.IsLocalDir(ctx, c.source) {
		c.source = c.source.AsDirectoryish()
	}
	if c.recursive || fs.IsLocalDir(ctx, c.dest) {
		c.dest = c.dest.AsDirectoryish()
	}

	if c.recursive {
		return c.moveRecursive(ctx, fs)
	}

	// if the destination is directoryish, we add the basename of the source
	// to the end of the destination to pick a filename.
	var base string
	if c.dest.Directoryish() && !c.source.Std() {
		// we undirectoryish the source so that we ignore any trailing slashes
		// when finding the base name.
		var ok bool
		base, ok = c.source.Undirectoryish().Base()
		if !ok {
			return errs.New("destination is a directory and cannot find base name for source %q", c.source)
		}
	}
	c.dest = joinDestWith(c.dest, base)

	return c.moveFile(ctx, fs, c.source, c.dest)
}

func (c *cmdMv) moveRecursive(ctx context.Context, fs ulfs.Filesystem) (err error) {
	defer mon.Task()(&ctx)(&err)

	iter, err := fs.List(ctx, c.source, &ulfs.ListOptions{
		Recursive: true,
	})
	if err != nil {
		return errs.Wrap(err)
	}

	var (
		limiter = sync2.NewLimiter(c.parallelism)
		es      errs.Group
		mu      sync.Mutex
	)

	fprintln := func(w io.Writer, args ...interface{}) {
		mu.Lock()
		defer mu.Unlock()

		_, _ = fmt.Fprintln(w, args...)
	}

	addError := func(err error) {
		mu.Lock()
		defer mu.Unlock()

		es.Add(err)
	}

	items := make([]ulfs.ObjectInfo, 0, 10)

	for iter.Next() {
		item := iter.Item()
		if item.IsPrefix {
			continue
		}

		items = append(items, item)
	}

	if err := iter.Err(); err != nil {
		return errs.Wrap(err)
	}

	for _, item := range items {
		source := item.Loc
		rel, err := c.source.RelativeTo(source)
		if err != nil {
			return err
		}
		dest := joinDestWith(c.dest, rel)

		ok := limiter.Go(ctx, func() {
			if c.progress {
				fprintln(clingy.Stdout(ctx), "Move", source, "to", dest)
			}

			if err := c.moveFile(ctx, fs, source, dest); err != nil {
				fprintln(clingy.Stdout(ctx), "Move", "failed:", err.Error())
				addError(err)
			}
		})
		if !ok {
			break
		}
	}

	limiter.Wait()

	if len(es) > 0 {
		return errs.Wrap(es.Err())
	}
	return nil
}

func (c *cmdMv) moveFile(ctx context.Context, fs ulfs.Filesystem, source, dest ulloc.Location) (err error) {
	defer mon.Task()(&ctx)(&err)

	if c.dryrun {
		return nil
	}

	return errs.Wrap(fs.Move(ctx, source, dest))
}
