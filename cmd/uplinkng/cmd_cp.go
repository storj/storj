// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"io"
	"strconv"
	"sync"

	progressbar "github.com/cheggaaa/pb/v3"
	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/common/ranger/httpranger"
	"storj.io/common/sync2"
	"storj.io/storj/cmd/uplinkng/ulext"
	"storj.io/storj/cmd/uplinkng/ulfs"
	"storj.io/storj/cmd/uplinkng/ulloc"
)

type cmdCp struct {
	ex ulext.External

	access      string
	recursive   bool
	parallelism int
	dryrun      bool
	progress    bool
	byteRange   string

	source ulloc.Location
	dest   ulloc.Location
}

func newCmdCp(ex ulext.External) *cmdCp {
	return &cmdCp{ex: ex}
}

func (c *cmdCp) Setup(params clingy.Parameters) {
	c.access = params.Flag("access", "Access name or value to use", "").(string)
	c.recursive = params.Flag("recursive", "Peform a recursive copy", false,
		clingy.Short('r'),
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.parallelism = params.Flag("parallelism", "Controls how many uploads/downloads to perform in parallel", 1,
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
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.progress = params.Flag("progress", "Show a progress bar when possible", true,
		clingy.Transform(strconv.ParseBool),
	).(bool)
	c.byteRange = params.Flag("range", "Downloads the specified range bytes of an object. For more information about the HTTP Range header, see https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.35", "").(string)

	c.source = params.Arg("source", "Source to copy", clingy.Transform(ulloc.Parse)).(ulloc.Location)
	c.dest = params.Arg("dest", "Destination to copy", clingy.Transform(ulloc.Parse)).(ulloc.Location)
}

func (c *cmdCp) Execute(ctx clingy.Context) error {
	if c.parallelism > 1 && c.byteRange != "" {
		return errs.New("parallelism and range flags are mutually exclusive")
	}
	fs, err := c.ex.OpenFilesystem(ctx, c.access)
	if err != nil {
		return err
	}
	defer func() { _ = fs.Close() }()

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
		return c.copyRecursive(ctx, fs)
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

	if !c.source.Std() && !c.dest.Std() {
		fmt.Fprintln(ctx.Stdout(), copyVerb(c.source, c.dest), c.source, "to", c.dest)
	}

	return c.copyFile(ctx, fs, c.source, c.dest, c.progress)
}

func (c *cmdCp) copyRecursive(ctx clingy.Context, fs ulfs.Filesystem) error {
	if c.source.Std() || c.dest.Std() {
		return errs.New("cannot recursively copy to stdin/stdout")
	}

	iter, err := fs.List(ctx, c.source, &ulfs.ListOptions{
		Recursive: true,
	})
	if err != nil {
		return err
	}

	var (
		limiter = sync2.NewLimiter(c.parallelism)
		es      errs.Group
		mu      sync.Mutex
	)

	fprintln := func(w io.Writer, args ...interface{}) {
		mu.Lock()
		defer mu.Unlock()

		fmt.Fprintln(w, args...)
	}

	addError := func(err error) {
		mu.Lock()
		defer mu.Unlock()

		es.Add(err)
	}

	for iter.Next() {
		source := iter.Item().Loc
		rel, err := c.source.RelativeTo(source)
		if err != nil {
			return err
		}
		dest := joinDestWith(c.dest, rel)

		ok := limiter.Go(ctx, func() {
			fprintln(ctx.Stdout(), copyVerb(source, dest), source, "to", dest)

			if err := c.copyFile(ctx, fs, source, dest, false); err != nil {
				fprintln(ctx.Stderr(), copyVerb(source, dest), "failed:", err.Error())
				addError(err)
			}
		})
		if !ok {
			break
		}
	}

	limiter.Wait()

	if err := iter.Err(); err != nil {
		return errs.Wrap(err)
	} else if len(es) > 0 {
		return es.Err()
	}
	return nil
}

func (c *cmdCp) copyFile(ctx clingy.Context, fs ulfs.Filesystem, source, dest ulloc.Location, progress bool) error {
	if c.dryrun {
		return nil
	}

	var length int64
	var openOpts *ulfs.OpenOptions

	if c.byteRange != "" {
		// TODO: we might want to avoid this call if ranged download will be used frequently
		stat, err := fs.Stat(ctx, source)
		if err != nil {
			return err
		}
		byteRange, err := httpranger.ParseRange(c.byteRange, stat.ContentLength)
		if err != nil && byteRange == nil {
			return errs.New("error parsing byte range %q: %w", c.byteRange, err)
		}
		if len(byteRange) == 0 {
			return errs.New("invalid range")
		}
		if len(byteRange) > 1 {
			return errs.New("retrieval of multiple byte ranges of data not supported: %d provided", len(byteRange))
		}

		length = byteRange[0].Length

		openOpts = &ulfs.OpenOptions{
			Offset: byteRange[0].Start,
			Length: byteRange[0].Length,
		}
	}

	rh, err := fs.Open(ctx, source, openOpts)
	if err != nil {
		return err
	}
	defer func() { _ = rh.Close() }()

	if length == 0 {
		length = rh.Info().ContentLength
	}

	wh, err := fs.Create(ctx, dest)
	if err != nil {
		return err
	}
	defer func() { _ = wh.Abort() }()

	var bar *progressbar.ProgressBar
	var writer io.Writer = wh

	if progress && length >= 0 && !c.dest.Std() {
		bar = progressbar.New64(length).SetWriter(ctx.Stdout())
		writer = bar.NewProxyWriter(writer)
		bar.Start()
		defer bar.Finish()
	}

	if _, err := io.Copy(writer, rh); err != nil {
		return errs.Combine(err, wh.Abort())
	}
	return errs.Wrap(wh.Commit())
}

func copyVerb(source, dest ulloc.Location) string {
	switch {
	case dest.Remote():
		return "upload"
	case source.Remote():
		return "download"
	default:
		return "copy"
	}
}

func joinDestWith(dest ulloc.Location, suffix string) ulloc.Location {
	dest = dest.AppendKey(suffix)
	// if the destination is local and directoryish, remove any
	// trailing slashes that it has. this makes it so that if
	// a remote file is name "foo/", then we copy it down as
	// just "foo".
	if dest.Local() && dest.Directoryish() {
		dest = dest.Undirectoryish()
	}
	return dest
}
