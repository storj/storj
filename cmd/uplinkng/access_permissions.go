// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplinkng/ulloc"
	"storj.io/uplink"
)

// accessPermissions holds flags and provides a Setup method for commands that
// have to modify permissions on access grants.
type accessPermissions struct {
	prefixes []uplink.SharePrefix // prefixes is the set of path prefixes that the grant will be limited to

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
	transformSharePrefix := func(loc ulloc.Location) (uplink.SharePrefix, error) {
		bucket, key, ok := loc.RemoteParts()
		if !ok {
			return uplink.SharePrefix{}, errs.New("invalid prefix: must be remote: %q", loc)
		}
		return uplink.SharePrefix{
			Bucket: bucket,
			Prefix: key,
		}, nil
	}

	ap.prefixes = params.Flag("prefix", "Key prefix access will be restricted to", []ulloc.Location{},
		clingy.Transform(ulloc.Parse),
		clingy.Transform(transformSharePrefix),
		clingy.Repeated,
	).([]uplink.SharePrefix)

	ap.readonly = params.Flag("readonly", "Implies --disallow-writes and --disallow-deletes", false,
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

	now := time.Now()
	transformHumanDate := clingy.Transform(func(date string) (time.Time, error) {
		switch {
		case date == "":
			return time.Time{}, nil
		case date == "now":
			return now, nil
		case date[0] == '+' || date[0] == '-':
			d, err := time.ParseDuration(date)
			return now.Add(d), errs.Wrap(err)
		default:
			t, err := time.Parse(time.RFC3339, date)
			return t, errs.Wrap(err)
		}
	})

	ap.notBefore = params.Flag("not-before",
		"Disallow access before this time (e.g. '+2h', 'now', '2020-01-02T15:04:05Z0700')",
		time.Time{}, transformHumanDate, clingy.Type("relative_date")).(time.Time)
	ap.notAfter = params.Flag("not-after",
		"Disallow access after this time (e.g. '+2h', 'now', '2020-01-02T15:04:05Z0700')",
		time.Time{}, transformHumanDate, clingy.Type("relative_date")).(time.Time)
}

func (ap *accessPermissions) Apply(access *uplink.Access) (*uplink.Access, error) {
	permission := uplink.Permission{
		AllowDelete:   !ap.disallowDeletes && !ap.readonly,
		AllowList:     !ap.disallowLists && !ap.writeonly,
		AllowDownload: !ap.disallowReads && !ap.writeonly,
		AllowUpload:   !ap.disallowWrites && !ap.readonly,
		NotBefore:     ap.notBefore,
		NotAfter:      ap.notAfter,
	}

	access, err := access.Share(permission, ap.prefixes...)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return access, nil
}
