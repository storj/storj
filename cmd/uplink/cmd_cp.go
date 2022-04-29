// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	progressbar "github.com/cheggaaa/pb/v3"
	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/common/context2"
	"storj.io/common/memory"
	"storj.io/common/rpc/rpcpool"
	"storj.io/common/sync2"
	"storj.io/storj/cmd/uplink/ulext"
	"storj.io/storj/cmd/uplink/ulfs"
	"storj.io/storj/cmd/uplink/ulloc"
)

type cmdCp struct {
	ex ulext.External

	access    string
	recursive bool
	transfers int
	dryrun    bool
	progress  bool
	byteRange string
	expires   time.Time

	parallelism          int
	parallelismChunkSize memory.Size

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
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.transfers = params.Flag("transfers", "Controls how many uploads/downloads to perform in parallel", 1,
		clingy.Short('t'),
		clingy.Transform(strconv.Atoi),
		clingy.Transform(func(n int) (int, error) {
			if n <= 0 {
				return 0, errs.New("transfers must be at least 1")
			}
			return n, nil
		}),
	).(int)
	c.dryrun = params.Flag("dry-run", "Print what operations would happen but don't execute them", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.progress = params.Flag("progress", "Show a progress bar when possible", true,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.byteRange = params.Flag("range", "Downloads the specified range bytes of an object. For more information about the HTTP Range header, see https://www.w3.org/Protocols/rfc2616/rfc2616-sec14.html#sec14.35", "").(string)

	c.parallelism = params.Flag("parallelism", "Controls how many parallel chunks to upload/download from a file", 1,
		clingy.Short('p'),
		clingy.Transform(strconv.Atoi),
		clingy.Transform(func(n int) (int, error) {
			if n <= 0 {
				return 0, errs.New("parallelism must be at least 1")
			}
			return n, nil
		}),
	).(int)
	c.parallelismChunkSize = params.Flag("parallelism-chunk-size", "Controls the size of the chunks for parallelism", 64*memory.MiB,
		clingy.Transform(memory.ParseString),
		clingy.Transform(func(n int64) (memory.Size, error) {
			if memory.Size(n) < 1*memory.MB {
				return 0, errs.New("file chunk size must be at least 1 MB")
			}
			return memory.Size(n), nil
		}),
	).(memory.Size)

	c.expires = params.Flag("expires",
		"Schedule removal after this time (e.g. '+2h', 'now', '2020-01-02T15:04:05Z0700')",
		time.Time{}, clingy.Transform(parseHumanDate), clingy.Type("relative_date")).(time.Time)

	c.source = params.Arg("source", "Source to copy", clingy.Transform(ulloc.Parse)).(ulloc.Location)
	c.dest = params.Arg("dest", "Destination to copy", clingy.Transform(ulloc.Parse)).(ulloc.Location)
}

func (c *cmdCp) Execute(ctx clingy.Context) error {
	fs, err := c.ex.OpenFilesystem(ctx, c.access, ulext.ConnectionPoolOptions(rpcpool.Options{
		Capacity:       100 * c.parallelism,
		KeyCapacity:    5,
		IdleExpiration: 2 * time.Minute,
	}))
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
		if c.byteRange != "" {
			return errs.New("unable to do recursive copy with byte range")
		}
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
		limiter = sync2.NewLimiter(c.transfers)
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

	if dest.Remote() && source.Remote() {
		return fs.Copy(ctx, source, dest)
	}

	offset, length, err := parseRange(c.byteRange)
	if err != nil {
		return errs.Wrap(err)
	}

	mrh, err := fs.Open(ctx, source)
	if err != nil {
		return err
	}
	defer func() { _ = mrh.Close() }()

	mwh, err := fs.Create(ctx, dest, &ulfs.CreateOptions{Expires: c.expires})
	if err != nil {
		return err
	}
	defer func() { _ = mwh.Abort(ctx) }()

	var bar *progressbar.ProgressBar
	if progress && !c.dest.Std() {
		bar = progressbar.New64(0).SetWriter(ctx.Stdout())
		defer bar.Finish()
	}

	return errs.Wrap(parallelCopy(
		ctx,
		mwh, mrh,
		c.parallelism, c.parallelismChunkSize.Int64(),
		offset, length,
		bar,
	))
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

func parallelCopy(
	clctx clingy.Context,
	dst ulfs.MultiWriteHandle,
	src ulfs.MultiReadHandle,
	p int, chunkSize int64,
	offset, length int64,
	bar *progressbar.ProgressBar) error {

	if offset != 0 {
		if err := src.SetOffset(offset); err != nil {
			return err
		}
	}

	var (
		limiter = sync2.NewLimiter(p)
		es      errs.Group
		mu      sync.Mutex
	)

	ctx, cancel := context.WithCancel(clctx)

	defer limiter.Wait()
	defer func() { _ = src.Close() }()
	defer func() {
		nocancel := context2.WithoutCancellation(ctx)
		timedctx, cancel := context.WithTimeout(nocancel, 5*time.Second)
		defer cancel()
		_ = dst.Abort(timedctx)
	}()
	defer cancel()

	for i := 0; length != 0; i++ {
		i := i

		chunk := chunkSize
		if length > 0 && chunkSize > length {
			chunk = length
		}
		length -= chunk

		rh, err := src.NextPart(ctx, chunk)
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			mu.Lock()
			fmt.Fprintln(clctx.Stderr(), "Error getting reader for part", i)
			mu.Unlock()

			return err
		}

		wh, err := dst.NextPart(ctx, chunk)
		if err != nil {
			_ = rh.Close()

			mu.Lock()
			fmt.Fprintln(clctx.Stderr(), "Error getting writer for part", i)
			mu.Unlock()

			return err
		}

		ok := limiter.Go(ctx, func() {
			defer func() { _ = rh.Close() }()
			defer func() { _ = wh.Abort() }()

			var w io.Writer = wh
			if bar != nil {
				bar.SetTotal(rh.Info().ContentLength).Start()
				w = bar.NewProxyWriter(w)
			}

			_, err := sync2.Copy(ctx, w, rh)
			if err == nil {
				err = wh.Commit()
			}

			if err != nil {
				// abort all other concurrenty copies
				cancel()
				// TODO: it would be also nice to use wh.Abort and rh.Close directly
				// to avoid some of the waiting that's caused by sync2.Copy.
				//
				// However, some of the source / destination implementations don't seem
				// to have concurrent safe API with that regards.
				//
				// Also, we may want to check that it actually helps, before implementing it.
			}

			mu.Lock()
			defer mu.Unlock()

			es.Add(err)
		})
		if !ok {
			mu.Lock()
			fmt.Fprintln(clctx.Stderr(), "Failed to start part copy", i)
			mu.Unlock()
			return ctx.Err()
		}
	}

	limiter.Wait()

	es.Add(dst.Commit(ctx))

	return es.Err()
}

func parseRange(r string) (offset, length int64, err error) {
	r = strings.TrimPrefix(strings.TrimSpace(r), "bytes=")
	if r == "" {
		return 0, -1, nil
	}

	if strings.Contains(r, ",") {
		return 0, 0, errs.New("invalid range: must be single range")
	}

	idx := strings.Index(r, "-")
	if idx < 0 {
		return 0, 0, errs.New(`invalid range: no "-"`)
	}

	start, end := strings.TrimSpace(r[:idx]), strings.TrimSpace(r[idx+1:])

	var starti, endi int64

	if start != "" {
		starti, err = strconv.ParseInt(start, 10, 64)
		if err != nil {
			return 0, 0, errs.New("invalid range: %w", err)
		}
	}

	if end != "" {
		endi, err = strconv.ParseInt(end, 10, 64)
		if err != nil {
			return 0, 0, errs.New("invalid range: %w", err)
		}
	}

	switch {
	case start == "" && end == "":
		return 0, 0, errs.New("invalid range")
	case start == "":
		return -endi, -1, nil
	case end == "":
		return starti, -1, nil
	case starti < 0:
		return 0, 0, errs.New("invalid range: negative start: %q", start)
	case starti > endi:
		return 0, 0, errs.New("invalid range: %v > %v", starti, endi)
	default:
		return starti, endi - starti + 1, nil
	}
}
