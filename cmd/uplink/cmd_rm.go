// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/hex"
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

type cmdRm struct {
	ex ulext.External

	access           string
	recursive        bool
	parallelism      int
	encrypted        bool
	pending          bool
	version          []byte
	bypassGovernance bool

	location ulloc.Location
}

func newCmdRm(ex ulext.External) *cmdRm {
	return &cmdRm{ex: ex}
}

func (c *cmdRm) Setup(params clingy.Parameters) {
	c.access = params.Flag("access", "Access name or value to use", "").(string)
	c.recursive = params.Flag("recursive", "Remove recursively", false,
		clingy.Short('r'),
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.parallelism = params.Flag("parallelism", "Controls how many removes to perform in parallel", 1,
		clingy.Short('p'),
		clingy.Transform(strconv.Atoi),
		clingy.Transform(func(n int) (int, error) {
			if n <= 0 {
				return 0, errs.New("parallelism must be at least 1")
			}
			return n, nil
		}),
	).(int)
	c.encrypted = params.Flag("encrypted", "Interprets keys base64 encoded without decrypting", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.pending = params.Flag("pending", "Remove pending object uploads instead", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.version = params.Flag("version-id", "Version ID to remove (if the location is an object path)", nil,
		clingy.Transform(hex.DecodeString),
	).([]byte)
	c.bypassGovernance = params.Flag("bypass-governance-retention", "Bypass Object Lock governance mode restrictions", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)

	c.location = params.Arg("location", "Location to remove (sj://BUCKET[/KEY])",
		clingy.Transform(ulloc.Parse),
	).(ulloc.Location)
}

func (c *cmdRm) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if c.location.Local() {
		return errs.New("remove %v skipped: local delete", c.location)
	}

	if c.version != nil {
		if c.recursive {
			return errs.New("a version ID must not be provided when deleting recursively")
		}
		if c.pending {
			return errs.New("a version ID must not be provided when deleting pending uploads")
		}
	}

	fs, err := c.ex.OpenFilesystem(ctx, c.access, ulext.BypassEncryption(c.encrypted))
	if err != nil {
		return err
	}
	defer func() { _ = fs.Close() }()

	if !c.recursive {
		err := fs.Remove(ctx, c.location, &ulfs.RemoveOptions{
			Pending:                   c.pending,
			Version:                   c.version,
			BypassGovernanceRetention: c.bypassGovernance,
		})
		if err != nil {
			return err
		}

		locStr := c.location.String()
		if c.version != nil {
			locStr += " version " + hex.EncodeToString(c.version)
		}

		_, _ = fmt.Fprintln(clingy.Stdout(ctx), "removed", locStr)
		return nil
	}

	iter, err := fs.List(ctx, c.location, &ulfs.ListOptions{
		Recursive: true,
		Pending:   c.pending,
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

		_, _ = fmt.Fprintln(w, args...)
	}

	addError := func(err error) {
		mu.Lock()
		defer mu.Unlock()

		es.Add(err)
	}

	for iter.Next() {
		loc := iter.Item().Loc

		ok := limiter.Go(ctx, func() {
			err := fs.Remove(ctx, loc, &ulfs.RemoveOptions{
				Pending: c.pending,
			})
			if err != nil {
				fprintln(clingy.Stderr(ctx), "remove", loc, "failed:", err.Error())
				addError(err)
			} else {
				fprintln(clingy.Stdout(ctx), "removed", loc)
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
