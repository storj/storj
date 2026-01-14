// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build linux
// +build linux

package main

import (
	"os"

	"github.com/zeebo/errs"

	"storj.io/storj/storagenode/hashstore"
	"storj.io/storj/storagenode/hashstore/platform"
)

type file struct {
	fh    *os.File
	m     []byte
	unmap bool
}

func openFile(name string) (_ *file, err error) {
	fh, err := os.OpenFile(name, os.O_RDWR, 0)
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
	if stat.Size() == 0 {
		return &file{
			fh:    fh,
			unmap: false,
		}, nil
	}
	m, err := platform.Mmap(fh, int(stat.Size()))
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &file{
		fh:    fh,
		m:     m,
		unmap: true,
	}, nil
}

func (f *file) Close() error {
	var eg errs.Group
	if f.unmap {
		eg.Add(platform.Munmap(f.m))
	}
	eg.Add(f.fh.Close())
	return eg.Err()
}

func (f *file) Size() int64 {
	return int64(len(f.m))
}

func (f *file) Record(off int64, rec *hashstore.Record) (ok bool, err error) {
	return rec.ReadFrom((*[hashstore.RecordSize]byte)(f.m[off:])), nil
}

func (f *file) ReadAt(p []byte, off int64) (n int, err error) {
	return copy(p, f.m[off:]), nil
}
