// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"github.com/skyrings/skyring-common/tools/uuid"

	"errors"
	"strconv"
	"strings"
)

// Path represents a object path
type Path = string

// SplitPath splits path into a slice of path components
func SplitPath(path Path) []string {
	return strings.Split(path, "/")
}

// JoinPaths concatenates paths to a new single path
func JoinPaths(paths ...Path) Path {
	return strings.Join(paths, "/")
}

// CreatePath will create a Segment path
func CreatePath(projectID uuid.UUID, segmentIndex int64, bucket, path []byte) (Path, error) {
	if segmentIndex < -1 {
		return "", errors.New("invalid segment index")
	}
	segment := "l"
	if segmentIndex > -1 {
		segment = "s" + strconv.FormatInt(segmentIndex, 10)
	}

	entries := make([]string, 0)
	entries = append(entries, projectID.String())
	entries = append(entries, segment)
	if len(bucket) != 0 {
		entries = append(entries, string(bucket))
	}
	if len(path) != 0 {
		entries = append(entries, string(path))
	}
	return JoinPaths(entries...), nil
}
