// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"os"

	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulloc"
)

//
// read handles
//

// osMultiReadHandle implements MultiReadHandle for *os.Files.
func newOSMultiReadHandle(fh *os.File) (MultiReadHandle, error) {
	fi, err := fh.Stat()
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return NewGenericMultiReadHandle(fh, ObjectInfo{
		Loc:           ulloc.NewLocal(fh.Name()),
		IsPrefix:      false,
		Created:       fi.ModTime(), // TODO: os specific crtime
		ContentLength: fi.Size(),
	}), nil
}

//
// write handles
//

type fileGenericWriter os.File

func (f *fileGenericWriter) raw() *os.File { return (*os.File)(f) }

func (f *fileGenericWriter) WriteAt(b []byte, off int64) (int, error) { return f.raw().WriteAt(b, off) }
func (f *fileGenericWriter) Commit() error                            { return f.raw().Close() }
func (f *fileGenericWriter) Abort() error {
	return errs.Combine(
		f.raw().Close(),
		os.Remove(f.raw().Name()),
	)
}

func newOSMultiWriteHandle(fh *os.File) MultiWriteHandle {
	return NewGenericMultiWriteHandle((*fileGenericWriter)(fh))
}
