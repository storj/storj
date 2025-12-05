// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/fs"
	"math/bits"
	"os"
	"path/filepath"
	"strconv"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/storj/storagenode/hashstore"
	"storj.io/storj/storagenode/hashstore/platform"
)

func main() {
	ok, err := clingy.Environment{
		Name: "write-hashtbl",
		Args: os.Args[1:],
		Root: newCmdRoot(),
	}.Run(context.Background(), nil)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%+v\n", err)
	}
	if !ok || err != nil {
		os.Exit(1)
	}
}

type cmdRoot struct {
	fast  bool
	slots *uint64
	kind  hashstore.TableKindCfg

	dir string

	buf []byte // reused piece buffer
}

func newCmdRoot() *cmdRoot { return &cmdRoot{} }

func (c *cmdRoot) Setup(params clingy.Parameters) {
	c.fast = params.Flag("fast", "Skip some checks for faster processing", false,
		clingy.Short('f'),
		clingy.Transform(strconv.ParseBool), clingy.Boolean,
	).(bool)
	c.slots = params.Flag("slots", "logSlots to use instead of counting", nil,
		clingy.Short('s'),
		clingy.Transform(func(s string) (uint64, error) {
			return strconv.ParseUint(s, 10, 64)
		}),
		clingy.Optional,
	).(*uint64)
	c.kind = params.Flag("kind", "Kind of table to write", hashstore.TableKindCfg{Kind: hashstore.TableKind_HashTbl},
		clingy.Short('k'),
	).(hashstore.TableKindCfg)

	c.dir = params.Arg("dir", "Directory containing log files to process").(string)
}

func (c *cmdRoot) Execute(ctx context.Context) (err error) {
	files, err := allFiles(c.dir)
	if err != nil {
		return errs.Wrap(err)
	}

	if c.slots == nil {
		var count uint64
		for _, file := range files {
			_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Counting %s...\n", file)
			c, err := c.countRecords(ctx, file)
			if err != nil {
				return errs.Wrap(err)
			}
			count += c
		}

		_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Record count=%d\n", count)

		slots := uint64(bits.Len64(count)) + 1
		c.slots = &slots
	}

	_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Using logSlots=%d\n", *c.slots)

	fh, err := platform.CreateFile("hashtbl")
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = fh.Close() }()

	// TODO: use injected configuration
	tcons, err := hashstore.CreateTable(ctx, fh, *c.slots, 0, c.kind.Kind, hashstore.CreateDefaultConfig(hashstore.TableKind_HashTbl, false))
	if err != nil {
		return errs.Wrap(err)
	}
	defer tcons.Cancel()

	for _, file := range files {
		_, _ = fmt.Fprintf(clingy.Stdout(ctx), "Processing %s...\n", file)
		err := c.iterateRecords(ctx, file, c.fast, func(rec hashstore.Record) error {
			ok, err := tcons.Append(ctx, rec)
			if err != nil {
				return err
			} else if !ok {
				return errs.New("Table too small. Try again with `-s %d`", *c.slots+1)
			}
			return nil
		})
		if err != nil {
			return errs.Wrap(err)
		}
	}

	tbl, err := tcons.Done(ctx)
	if err != nil {
		return err
	}
	return tbl.Close()
}

func isLogFile(path string) (uint64, bool) {
	name := filepath.Base(path)
	if (len(name) != 20 && len(name) != 29) || name[0:4] != "log-" {
		return 0, false
	}
	id, err := strconv.ParseUint(name[4:20], 16, 64)
	if err != nil {
		return 0, false
	}
	return id, true
}

func (c *cmdRoot) iterateRecords(ctx context.Context, path string, fast bool, cb func(rec hashstore.Record) error) (err error) {
	id, ok := isLogFile(path)
	if !ok {
		return nil
	}

	file, err := openFile(path)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() { _ = file.Close() }()

	var rec hashstore.Record

	for i := file.Size() - hashstore.RecordSize; i >= 0; i-- {
		if ok, err := file.Record(i, &rec); err != nil {
			return errs.Wrap(err)
		} else if !ok {
			continue
		}

		if rec.Log != id {
			_, _ = fmt.Fprintf(clingy.Stderr(ctx), "record at offset=%d in %s has invalid id=%d\n", i, path, rec.Log)
			continue
		}

		if rec.Offset+uint64(rec.Length) != uint64(i) {
			_, _ = fmt.Fprintf(clingy.Stderr(ctx), "record at offset=%d in %s has invalid offset=%d length=%d\n", i, path, rec.Offset, rec.Length)
			continue
		}

		if !fast {
			if len(c.buf) < int(rec.Length) {
				c.buf = make([]byte, rec.Length)
			}

			_, err := file.ReadAt(c.buf[:rec.Length], int64(rec.Offset))
			if err != nil {
				return errs.Wrap(err)
			}

			pieceData := c.buf[:rec.Length-512]
			headerData := c.buf[rec.Length-512 : rec.Length]

			l := binary.BigEndian.Uint16(headerData[0:2])
			var header pb.PieceHeader
			if err := pb.Unmarshal(headerData[2:2+l], &header); err != nil {
				_, _ = fmt.Fprintf(clingy.Stderr(ctx), "record at offset=%d in %s has invalid piece header with err=%v\n", i, path, err)
				continue
			}

			h := pb.NewHashFromAlgorithm(header.GetHashAlgorithm())
			h.Write(pieceData)
			if sum := h.Sum(nil); string(sum) != string(header.Hash) {
				_, _ = fmt.Fprintf(clingy.Stderr(ctx), "record at offset=%d in %s has invalid piece hash=%x\n", i, path, sum)
				continue
			}
		}

		if err := cb(rec); err != nil {
			return errs.Wrap(err)
		}

		i = int64(rec.Offset) - hashstore.RecordSize + 1
	}

	return nil
}

func (c *cmdRoot) countRecords(ctx context.Context, path string) (n uint64, err error) {
	err = c.iterateRecords(ctx, path, true, func(rec hashstore.Record) error {
		n++
		return nil
	})
	return n, err
}

// allFiles recursively collects all files in the given directory and returns
// their full path.
func allFiles(dir string) (paths []string, err error) {
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if _, ok := isLogFile(path); ok {
				paths = append(paths, path)
			}
		}
		return err
	})
	return paths, errs.Wrap(err)
}
