// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"
	"time"

	"github.com/zeebo/clingy"
)

// accessPermissions holds flags and provides a Setup method for commands that
// have to modify permissions on access grants.
type accessPermissions struct {
	paths []string // paths is the set of path prefixes that the grant will be limited to

	readonly  bool // implies disallowWrites and disallowDeletes
	writeonly bool // implies disallowReads and disallowLists

	disallowDeletes bool
	disallowLists   bool
	disallowReads   bool
	disallowWrites  bool

	notBefore time.Time
	notAfter  time.Time
}

func (ap *accessPermissions) Setup(a clingy.Arguments, f clingy.Flags) {
	ap.paths = f.New("path", "Path prefix access will be restricted to", []string{},
		clingy.Repeated).([]string)

	ap.readonly = f.New("readonly", "Implies --disallow-writes and --disallow-deletes", true,
		clingy.Transform(strconv.ParseBool)).(bool)
	ap.writeonly = f.New("writeonly", "Implies --disallow-reads and --disallow-lists", false,
		clingy.Transform(strconv.ParseBool)).(bool)

	ap.disallowDeletes = f.New("disallow-deletes", "Disallow deletes with the access", false,
		clingy.Transform(strconv.ParseBool)).(bool)
	ap.disallowLists = f.New("disallow-lists", "Disallow lists with the access", false,
		clingy.Transform(strconv.ParseBool)).(bool)
	ap.disallowReads = f.New("disallow-reads", "Disallow reasd with the access", false,
		clingy.Transform(strconv.ParseBool)).(bool)
	ap.disallowWrites = f.New("disallow-writes", "Disallow writes with the access", false,
		clingy.Transform(strconv.ParseBool)).(bool)

	ap.notBefore = f.New("not-before",
		"Disallow access before this time (e.g. '+2h', '2020-01-02T15:04:05Z0700')",
		time.Time{}, clingy.Transform(parseRelativeTime), clingy.Type("relative_time")).(time.Time)
	ap.notAfter = f.New("not-after",
		"Disallow access after this time (e.g. '+2h', '2020-01-02T15:04:05Z0700')",
		time.Time{}, clingy.Transform(parseRelativeTime), clingy.Type("relative_time")).(time.Time)
}

func parseRelativeTime(v string) (time.Time, error) {
	if len(v) == 0 {
		return time.Time{}, nil
	} else if v[0] == '+' || v[0] == '-' {
		d, err := time.ParseDuration(v)
		if err != nil {
			return time.Time{}, err
		}
		return time.Now().Add(d), nil
	} else {
		return time.Parse(time.RFC3339, v)
	}
}
