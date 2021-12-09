// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package ulfs

import (
	"os"

	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulloc"
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
