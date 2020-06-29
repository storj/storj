// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package objectdeletion

import (
	"errors"
	"strconv"
	"strings"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
)

// ObjectIdentifier contains information about an object
// that are needed for delete operation.
type ObjectIdentifier struct {
	ProjectID     uuid.UUID
	Bucket        []byte
	EncryptedPath []byte
}

// SegmentPath returns a raw path for a specific segment index.
func (id *ObjectIdentifier) SegmentPath(segmentIndex int64) ([]byte, error) {
	if segmentIndex < lastSegmentIndex {
		return nil, errors.New("invalid segment index")
	}
	segment := "l"
	if segmentIndex > lastSegmentIndex {
		segment = "s" + strconv.FormatInt(segmentIndex, 10)
	}

	return []byte(storj.JoinPaths(
		id.ProjectID.String(),
		segment,
		string(id.Bucket),
		string(id.EncryptedPath),
	)), nil
}

// ParseSegmentPath parses a raw path and returns an
// object identifier from that path along with the path's segment index.
// example: <project-id>/01/<bucket-name>/<encrypted-path>
func ParseSegmentPath(rawPath []byte) (ObjectIdentifier, int64, error) {
	elements := storj.SplitPath(string(rawPath))
	if len(elements) < 4 {
		return ObjectIdentifier{}, -1, errs.New("invalid path %q", string(rawPath))
	}

	projectID, err := uuid.FromString(elements[0])
	if err != nil {
		return ObjectIdentifier{}, -1, errs.Wrap(err)
	}
	var segmentIndex int64
	if elements[1] == "l" {
		segmentIndex = lastSegmentIndex
	} else {
		segmentIndex, err = strconv.ParseInt(elements[1][1:], 10, 64) // remove the strng `s` from segment index we got

		if err != nil {
			return ObjectIdentifier{}, -1, errs.Wrap(err)
		}
	}

	return ObjectIdentifier{
		ProjectID:     projectID,
		Bucket:        []byte(elements[2]),
		EncryptedPath: []byte(storj.JoinPaths(elements[3:]...)),
	}, segmentIndex, nil
}

// Key returns a string concatenated by all object identifier fields plus 0.
// It's a unique string used to identify an object.
// It's not a valid key for retrieving pointers from metainfo database.
func (id *ObjectIdentifier) Key() string {
	builder := strings.Builder{}
	// we don't need the return value here
	// Write will always return the length of the argument and nil error
	_, _ = builder.Write(id.ProjectID[:])
	_, _ = builder.Write(id.Bucket)
	_, _ = builder.Write(id.EncryptedPath)

	return builder.String()
}
