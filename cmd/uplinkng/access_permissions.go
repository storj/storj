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
	prefixes []string // prefixes is the set of path prefixes that the grant will be limited to

	readonly  bool // implies disallowWrites and disallowDeletes
	writeonly bool // implies disallowReads and disallowLists

	disallowDeletes bool
	disallowLists   bool
	disallowReads   bool
	disallowWrites  bool

	notBefore time.Time
	notAfter  time.Time
}

func (ap *accessPermissions) Setup(params clingy.Parameters) {
	ap.prefixes = params.Flag("prefix", "Key prefix access will be restricted to", []string{},
		clingy.Repeated).([]string)

	ap.readonly = params.Flag("readonly", "Implies --disallow-writes and --disallow-deletes", true,
		clingy.Transform(strconv.ParseBool)).(bool)
	ap.writeonly = params.Flag("writeonly", "Implies --disallow-reads and --disallow-lists", false,
		clingy.Transform(strconv.ParseBool)).(bool)

	ap.disallowDeletes = params.Flag("disallow-deletes", "Disallow deletes with the access", false,
		clingy.Transform(strconv.ParseBool)).(bool)
	ap.disallowLists = params.Flag("disallow-lists", "Disallow lists with the access", false,
		clingy.Transform(strconv.ParseBool)).(bool)
	ap.disallowReads = params.Flag("disallow-reads", "Disallow reasd with the access", false,
		clingy.Transform(strconv.ParseBool)).(bool)
	ap.disallowWrites = params.Flag("disallow-writes", "Disallow writes with the access", false,
		clingy.Transform(strconv.ParseBool)).(bool)

	ap.notBefore = params.Flag("not-before",
		"Disallow access before this time (e.g. '+2h', '2020-01-02T15:04:05Z0700')",
		time.Time{}, clingy.Transform(parseRelativeTime), clingy.Type("relative_time")).(time.Time)
	ap.notAfter = params.Flag("not-after",
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
