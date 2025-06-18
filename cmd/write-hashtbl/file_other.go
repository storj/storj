// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build !linux
// +build !linux

package main

import (
	"os"

	"github.com/zeebo/errs"

	"storj.io/storj/storagenode/hashstore"
)

type file struct {
	fh   *os.File
	size int64
}

func openFile(name string) (_ *file, err error) {
	fh, err := os.Open(name)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	defer func() {
		if err != nil {
			_ = fh.Close()
		}
	}()
	stat, err := fh.Stat()
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &file{
		fh:   fh,
		size: stat.Size(),
	}, nil
}

func (f *file) Close() error {
	return errs.Combine(
		f.fh.Close(),
	)
}

func (f *file) Size() int64 {
	return f.size
}

func (f *file) Record(off int64, rec *hashstore.Record) (ok bool, err error) {
	var buf [hashstore.RecordSize]byte
	if _, err := f.fh.ReadAt(buf[:], off); err != nil {
		return false, errs.Wrap(err)
	}
	return rec.ReadFrom(&buf), nil
}

func (f *file) ReadAt(p []byte, off int64) (n int, err error) {
	return f.fh.ReadAt(p, off)
}
