// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VividCortex/ewma"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/common/context2"
	"storj.io/common/fpath"
	"storj.io/common/memory"
	"storj.io/common/rpc/rpcpool"
	"storj.io/common/sync2"
	"storj.io/storj/cmd/uplink/ulext"
	"storj.io/storj/cmd/uplink/ulfs"
	"storj.io/storj/cmd/uplink/ulloc"
	"storj.io/uplink/private/testuplink"
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
	metadata  map[string]string

	parallelism          int
	parallelismChunkSize memory.Size

	uploadConfig  testuplink.ConcurrentSegmentUploadsConfig
	uploadLogFile string

	inmemoryEC bool

	locs []ulloc.Location
}

const maxPartCount int64 = 10000

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

	parallelism := params.Flag("parallelism", "Controls how many parallel chunks to upload/download from a file", nil,
		clingy.Optional,
		clingy.Short('p'),
		clingy.Transform(strconv.Atoi),
		clingy.Transform(func(n int) (int, error) {
			if n <= 0 {
				return 0, errs.New("parallelism must be at least 1")
			}
			return n, nil
		}),
	).(*int)
	c.parallelismChunkSize = params.Flag("parallelism-chunk-size", "Set the size of the chunks for parallelism, 0 means automatic adjustment", memory.Size(0),
		clingy.Transform(memory.ParseString),
		clingy.Transform(func(n int64) (memory.Size, error) {
			if n < 0 {
				return 0, errs.New("parallelism-chunk-size cannot be below 0")
			}
			return memory.Size(n), nil
		}),
	).(memory.Size)

	c.uploadConfig = testuplink.DefaultConcurrentSegmentUploadsConfig()
	maxConcurrent := params.Flag(
		"maximum-concurrent-pieces",
		"Maximum concurrent pieces to upload at once per transfer",
		nil,
		clingy.Optional,
		clingy.Transform(strconv.Atoi),
		clingy.Advanced,
	).(*int)
	c.uploadConfig.SchedulerOptions.MaximumConcurrentHandles = params.Flag(
		"maximum-concurrent-segments",
		"Maximum concurrent segments to upload at once per transfer",
		c.uploadConfig.SchedulerOptions.MaximumConcurrentHandles,
		clingy.Transform(strconv.Atoi),
		clingy.Advanced,
	).(int)
	c.uploadConfig.LongTailMargin = params.Flag(
		"long-tail-margin",
		"How many extra pieces to upload and cancel per segment",
		c.uploadConfig.LongTailMargin,
		clingy.Transform(strconv.Atoi),
		clingy.Advanced,
	).(int)
	c.uploadLogFile = params.Flag("upload-log-file", "File to write upload logs to", "",
		clingy.Advanced,
	).(string)

	{ // handle backwards compatibility around parallelism and maximum concurrent pieces
		addr := func(x int) *int { return &x }

		switch {
		// if neither are actively set, use defaults
		case parallelism == nil && maxConcurrent == nil:
			parallelism = addr(1)
			maxConcurrent = addr(c.uploadConfig.SchedulerOptions.MaximumConcurrent)

		// if parallelism is not set, use a value based on maxConcurrent
		case parallelism == nil:
			parallelism = addr((*maxConcurrent + 99) / 100)

		// if maxConcurrent is not set, use a value based on parallelism
		case maxConcurrent == nil:
			maxConcurrent = addr(100 * *parallelism)
		}

		c.uploadConfig.SchedulerOptions.MaximumConcurrent = *maxConcurrent
		c.parallelism = *parallelism
	}

	c.inmemoryEC = params.Flag("inmemory-erasure-coding", "Keep erasure-coded pieces in-memory instead of writing them on the disk during upload", false,
		clingy.Transform(strconv.ParseBool),
		clingy.Boolean,
		clingy.Advanced,
	).(bool)

	c.expires = params.Flag("expires",
		"Schedule removal after this time (e.g. '+2h', 'now', '2020-01-02T15:04:05Z0700')",
		time.Time{}, clingy.Transform(parseHumanDate), clingy.Type("relative_date")).(time.Time)

	c.metadata = params.Flag("metadata",
		"optional metadata for the object. Please use a single level JSON object of string to string only",
		nil, clingy.Transform(parseJSON), clingy.Type("string")).(map[string]string)

	c.locs = params.Arg("locations", "Locations to copy (at least one source and one destination). Use - for standard input/output",
		clingy.Transform(ulloc.Parse),
		clingy.Repeated,
	).([]ulloc.Location)
}

func (c *cmdCp) Execute(ctx context.Context) error {
	if len(c.locs) < 2 {
		return errs.New("must have at least one source and destination path")
	}

	if c.uploadLogFile != "" {
		fh, err := os.OpenFile(c.uploadLogFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return errs.Wrap(err)
		}
		defer func() { _ = fh.Close() }()
		bw := bufio.NewWriter(fh)
		defer func() { _ = bw.Flush() }()
		ctx = testuplink.WithLogWriter(ctx, bw)
	}

	fs, err := c.ex.OpenFilesystem(ctx, c.access,
		ulext.ConcurrentSegmentUploadsConfig(c.uploadConfig),
		ulext.ConnectionPoolOptions(rpcpool.Options{
			// Add a bit more capacity for connections to the satellite
			Capacity:       c.uploadConfig.SchedulerOptions.MaximumConcurrent + 5,
			KeyCapacity:    5,
			IdleExpiration: 2 * time.Minute,
		}))
	if err != nil {
		return err
	}
	defer func() { _ = fs.Close() }()

	if c.inmemoryEC {
		ctx = fpath.WithTempData(ctx, "", true)
	}

	var eg errs.Group
	for _, source := range c.locs[:len(c.locs)-1] {
		eg.Add(c.dispatchCopy(ctx, fs, source, c.locs[len(c.locs)-1]))
	}
	return combineErrs(eg)
}

func (c *cmdCp) dispatchCopy(ctx context.Context, fs ulfs.Filesystem, source, dest ulloc.Location) error {
	if !source.Remote() && !dest.Remote() {
		return errs.New("at least one location must be a remote sj:// location")
	}

	// we ensure the source and destination are lexically directoryish
	// if they map to directories. the destination is always converted to be
	// directoryish if the copy is recursive or if there are multiple source paths
	if fs.IsLocalDir(ctx, source) {
		source = source.AsDirectoryish()
	}
	if c.recursive || len(c.locs) > 2 || fs.IsLocalDir(ctx, dest) {
		dest = dest.AsDirectoryish()
	}

	if c.recursive {
		if c.byteRange != "" {
			return errs.New("unable to do recursive copy with byte range")
		}
		return c.copyRecursive(ctx, fs, source, dest)
	}

	// if the destination is directoryish, we add the basename of the source
	// to the end of the destination to pick a filename.
	var base string
	if dest.Directoryish() && !source.Std() {
		// we undirectoryish the source so that we ignore any trailing slashes
		// when finding the base name.
		var ok bool
		base, ok = source.Undirectoryish().Base()
		if !ok {
			return errs.New("destination is a directory and cannot find base name for source %q", source)
		}
	}
	dest = joinDestWith(dest, base)

	if !dest.Std() {
		fmt.Fprintln(clingy.Stdout(ctx), copyVerb(source, dest), source, "to", dest)
	}

	var bar *mpb.Bar
	if c.progress && !dest.Std() {
		progress := mpb.New(mpb.WithOutput(clingy.Stdout(ctx)))
		defer progress.Wait()

		var namer barNamer
		bar = newProgressBar(progress, namer.NameFor(source, dest), 1, 1)
		defer func() {
			bar.Abort(true)
			bar.Wait()
		}()
	}

	return c.copyFile(ctx, fs, source, dest, bar)
}

func (c *cmdCp) copyRecursive(ctx context.Context, fs ulfs.Filesystem, source, dest ulloc.Location) error {
	if source.Std() || dest.Std() {
		return errs.New("cannot recursively copy to stdin/stdout")
	}

	iter, err := fs.List(ctx, source, &ulfs.ListOptions{
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
	fprintf := func(w io.Writer, format string, args ...interface{}) {
		mu.Lock()
		defer mu.Unlock()

		fmt.Fprintf(w, format, args...)
	}

	addError := func(err error) {
		if err == nil {
			return
		}

		mu.Lock()
		defer mu.Unlock()

		es.Add(err)
	}

	type copyOp struct {
		src  ulloc.Location
		dest ulloc.Location
	}

	var verb string
	var verbing string
	var namer barNamer
	var ops []copyOp
	for iter.Next() {
		item := iter.Item().Loc
		rel, err := source.RelativeTo(item)
		if err != nil {
			return err
		}
		dest := joinDestWith(dest, rel)

		verb = copyVerb(item, dest)
		verbing = copyVerbing(item, dest)

		namer.Preview(item, dest)

		ops = append(ops, copyOp{src: item, dest: dest})
	}
	if err := iter.Err(); err != nil {
		return errs.Wrap(err)
	}

	fprintln(clingy.Stdout(ctx), verbing, len(ops), "files...")

	var progress *mpb.Progress
	if c.progress {
		progress = mpb.New(mpb.WithOutput(clingy.Stdout(ctx)))
		defer progress.Wait()
	}

	for i, op := range ops {
		i := i
		op := op

		ok := limiter.Go(ctx, func() {
			var bar *mpb.Bar
			if progress != nil {
				bar = newProgressBar(progress, namer.NameFor(op.src, op.dest), i+1, len(ops))
				defer func() {
					bar.Abort(true)
					bar.Wait()
				}()
			} else {
				fprintf(clingy.Stdout(ctx), "%s %s to %s (%d of %d)\n", verb, op.src, op.dest, i+1, len(ops))
			}
			if err := c.copyFile(ctx, fs, op.src, op.dest, bar); err != nil {
				addError(errs.New("%s %s to %s failed: %w", verb, op.src, op.dest, err))
			}
		})
		if !ok {
			break
		}
	}

	limiter.Wait()

	if progress != nil {
		// Wait for all of the progress bar output to complete before doing
		// additional output.
		progress.Wait()
	}

	if len(es) > 0 {
		for _, e := range es {
			fprintln(clingy.Stdout(ctx), e)
		}
		return errs.New("recursive %s failed (%d of %d)", verb, len(es), len(ops))
	}
	return nil
}

func (c *cmdCp) copyFile(ctx context.Context, fs ulfs.Filesystem, source, dest ulloc.Location, bar *mpb.Bar) (err error) {
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

	mwh, err := fs.Create(ctx, dest, &ulfs.CreateOptions{
		Expires:  c.expires,
		Metadata: c.metadata,
	})
	if err != nil {
		return err
	}
	defer func() { _ = mwh.Abort(ctx) }()

	// if we're uploading, do a single part of maximum size
	if dest.Remote() {
		return errs.Wrap(c.singleCopy(
			ctx,
			source, dest,
			mrh, mwh,
			offset, length,
			bar,
		))
	}

	partSize, err := c.calculatePartSize(mrh.Length(), c.parallelismChunkSize.Int64())
	if err != nil {
		return err
	}

	return errs.Wrap(c.parallelCopy(
		ctx,
		source, dest,
		mrh, mwh,
		c.parallelism, partSize,
		offset, length,
		bar,
	))
}

// calculatePartSize returns the needed part size in order to upload the file with size of 'length'.
// It hereby respects if the client requests/prefers a certain size and only increases if needed.
func (c *cmdCp) calculatePartSize(length, preferredSize int64) (requiredSize int64, err error) {
	segC := (length / maxPartCount / (memory.MiB * 64).Int64()) + 1
	requiredSize = segC * (memory.MiB * 64).Int64()
	switch {
	case preferredSize == 0:
		return requiredSize, nil
	case requiredSize <= preferredSize:
		return preferredSize, nil
	default:
		return 0, errs.New(fmt.Sprintf("the specified chunk size %s is too small, requires %s or larger",
			memory.FormatBytes(preferredSize), memory.FormatBytes(requiredSize)))
	}
}

func copyVerbing(source, dest ulloc.Location) (verb string) {
	return copyVerb(source, dest) + "ing"
}

func copyVerb(source, dest ulloc.Location) (verb string) {
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

func (c *cmdCp) parallelCopy(
	ctx context.Context,
	source, dest ulloc.Location,
	src ulfs.MultiReadHandle,
	dst ulfs.MultiWriteHandle,
	p int, chunkSize int64,
	offset, length int64,
	bar *mpb.Bar) error {

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

	ctx, cancel := context.WithCancel(ctx)

	defer func() { _ = src.Close() }()
	defer func() {
		nocancel := context2.WithoutCancellation(ctx)
		timedctx, cancel := context.WithTimeout(nocancel, 5*time.Second)
		defer cancel()
		_ = dst.Abort(timedctx)
	}()
	defer cancel()

	addError := func(err error) {
		if err == nil {
			return
		}

		mu.Lock()
		defer mu.Unlock()

		es.Add(err)

		// abort all other concurrenty copies
		cancel()
	}

	var readBufs *ulfs.BytesPool
	if p > 1 && chunkSize > 0 && (source.Std() || dest.Std()) {
		// Create the read buffer pool only for uploads from stdin and downloads to stdout with parallelism > 1.
		readBufs = ulfs.NewBytesPool(int(chunkSize))
	}

	for i := 0; length != 0; i++ {
		i := i

		chunk := chunkSize
		if length > 0 && chunkSize > length {
			chunk = length
		}
		length -= chunk

		rh, err := src.NextPart(ctx, chunk)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				addError(errs.New("error getting reader for part %d: %v", i, err))
			}
			break
		}

		wh, err := dst.NextPart(ctx, chunk)
		if err != nil {
			_ = rh.Close()

			addError(errs.New("error getting writer for part %d: %v", i, err))
			break
		}

		ok := limiter.Go(ctx, func() {
			defer func() { _ = rh.Close() }()
			defer func() { _ = wh.Abort() }()

			if readBufs != nil {
				buf := readBufs.Get()
				defer readBufs.Put(buf)

				rh = ulfs.NewBufferedReadHandle(ctx, rh, buf)
			}

			var w io.Writer = wh
			if bar != nil {
				bar.SetTotal(rh.Info().ContentLength, false)
				bar.EnableTriggerComplete()
				pw := bar.ProxyWriter(w)
				defer func() {
					_ = pw.Close()
				}()
				w = pw
			}

			_, err := sync2.Copy(ctx, w, rh)
			if err == nil {
				err = wh.Commit()
			}

			if err != nil {
				// TODO: it would be also nice to use wh.Abort and rh.Close directly
				// to avoid some of the waiting that's caused by sync2.Copy.
				//
				// However, some of the source / destination implementations don't seem
				// to have concurrent safe API with that regards.
				//
				// Also, we may want to check that it actually helps, before implementing it.

				addError(errs.New("failed to %s part %d: %v", copyVerb(source, dest), i, err))
			}
		})
		if !ok {
			break
		}
	}

	limiter.Wait()

	// don't try to commit if any error occur
	if len(es) == 0 {
		es.Add(dst.Commit(ctx))
	}

	return errs.Wrap(combineErrs(es))
}

func (c *cmdCp) singleCopy(
	ctx context.Context,
	source, dest ulloc.Location,
	src ulfs.MultiReadHandle,
	dst ulfs.MultiWriteHandle,
	offset, length int64,
	bar *mpb.Bar) error {

	if offset != 0 {
		if err := src.SetOffset(offset); err != nil {
			return err
		}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	rh, err := src.NextPart(ctx, length)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = rh.Close() }()

	wh, err := dst.NextPart(ctx, length)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = wh.Abort() }()

	var w io.Writer = wh
	if bar != nil {
		bar.SetTotal(rh.Info().ContentLength, false)
		bar.EnableTriggerComplete()
		pw := bar.ProxyWriter(w)
		defer func() { _ = pw.Close() }()
		w = pw
	}

	if _, err := sync2.Copy(ctx, w, rh); err != nil {
		return errs.Wrap(err)
	}

	if err := wh.Commit(); err != nil {
		return errs.Wrap(err)
	}

	if err := dst.Commit(ctx); err != nil {
		return errs.Wrap(err)
	}

	return nil
}

func newProgressBar(progress *mpb.Progress, name string, which, total int) *mpb.Bar {
	const counterFmt = " % .2f / % .2f"
	const percentageFmt = "%.2f "
	const speedFmt = "% .2f"

	movingAverage := ewma.NewMovingAverage()

	prepends := []decor.Decorator{decor.Name(name + " ")}
	if total > 1 {
		prepends = append(prepends, decor.Name(fmt.Sprintf("(%d of %d)", which, total)))
	}
	prepends = append(prepends, decor.CountersKiloByte(counterFmt))

	appends := []decor.Decorator{
		decor.NewPercentage(percentageFmt),
		decor.MovingAverageSpeed(decor.SizeB1024(1024), speedFmt, movingAverage),
	}

	return progress.AddBar(0,
		mpb.PrependDecorators(prepends...),
		mpb.AppendDecorators(appends...),
	)
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

// combineErrs makes a more readable error message from the errors group.
func combineErrs(group errs.Group) error {
	if len(group) == 0 {
		return nil
	}

	errstrings := make([]string, len(group))
	for i, err := range group {
		errstrings[i] = err.Error()
	}

	return fmt.Errorf("%s", strings.Join(errstrings, "\n"))
}

type barNamer struct {
	longestTotalLen int
}

func (n *barNamer) Preview(src, dst ulloc.Location) {
	if src.Local() {
		n.preview(src)
		return
	}
	n.preview(dst)
}

func (n *barNamer) NameFor(src, dst ulloc.Location) string {
	if src.Local() {
		return n.nameFor(src)
	}
	return n.nameFor(dst)
}

func (n *barNamer) preview(loc ulloc.Location) {
	locLen := len(loc.String())
	if locLen > n.longestTotalLen {
		n.longestTotalLen = locLen
	}
}

func (n *barNamer) nameFor(loc ulloc.Location) string {
	name := loc.String()
	if n.longestTotalLen > 0 {
		pad := n.longestTotalLen - len(loc.String())
		name += strings.Repeat(" ", pad)
	}
	return name
}
