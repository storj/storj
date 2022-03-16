// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/ulloc"
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

func (ap *accessPermissions) Setup(params clingy.Parameters, prefixFlags bool) {
	if prefixFlags {
		ap.prefixes = params.Flag("prefix", "Key prefix access will be restricted to", []uplink.SharePrefix{},
			clingy.Transform(ulloc.Parse),
			clingy.Transform(transformSharePrefix),
			clingy.Repeated,
		).([]uplink.SharePrefix)
	}

	ap.readonly = params.Flag("readonly", "Implies --disallow-writes and --disallow-deletes", true,
		clingy.Transform(strconv.ParseBool), clingy.Boolean).(bool)
	ap.writeonly = params.Flag("writeonly", "Implies --disallow-reads and --disallow-lists", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean).(bool)

	ap.disallowDeletes = params.Flag("disallow-deletes", "Disallow deletes with the access", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean).(bool)
	ap.disallowLists = params.Flag("disallow-lists", "Disallow lists with the access", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean).(bool)
	ap.disallowReads = params.Flag("disallow-reads", "Disallow reasd with the access", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean).(bool)
	ap.disallowWrites = params.Flag("disallow-writes", "Disallow writes with the access", false,
		clingy.Transform(strconv.ParseBool), clingy.Boolean).(bool)

	ap.notBefore = params.Flag("not-before",
		"Disallow access before this time (e.g. '+2h', 'now', '2020-01-02T15:04:05Z0700')",
		time.Time{}, transformHumanDate, clingy.Type("relative_date")).(time.Time)
	ap.notAfter = params.Flag("not-after",
		"Disallow access after this time (e.g. '+2h', 'now', '2020-01-02T15:04:05Z0700')",
		time.Time{}, transformHumanDate, clingy.Type("relative_date")).(time.Time)

	if !prefixFlags {
		ap.prefixes = params.Arg("prefix", "Key prefix access will be restricted to",
			clingy.Transform(ulloc.Parse),
			clingy.Transform(transformSharePrefix),
			clingy.Repeated,
		).([]uplink.SharePrefix)
	}
}

func transformSharePrefix(loc ulloc.Location) (uplink.SharePrefix, error) {
	bucket, key, ok := loc.RemoteParts()
	if !ok {
		return uplink.SharePrefix{}, errs.New("invalid prefix: must be remote: %q", loc)
	}
	return uplink.SharePrefix{
		Bucket: bucket,
		Prefix: key,
	}, nil
}

func (ap *accessPermissions) Apply(access *uplink.Access) (*uplink.Access, error) {
	permission := uplink.Permission{
		AllowDelete:   ap.AllowDelete(),
		AllowList:     ap.AllowList(),
		AllowDownload: ap.AllowDownload(),
		AllowUpload:   ap.AllowUpload(),
		NotBefore:     ap.notBefore,
		NotAfter:      ap.notAfter,
	}

	// if we aren't actually restricting anything, then we don't need to Share.
	if permission == (uplink.Permission{
		AllowDelete:   true,
		AllowList:     true,
		AllowDownload: true,
		AllowUpload:   true,
	}) && len(ap.prefixes) == 0 {
		return access, nil
	}

	access, err := access.Share(permission, ap.prefixes...)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return access, nil
}

func (ap *accessPermissions) AllowDelete() bool {
	return !ap.disallowDeletes && !ap.readonly
}

func (ap *accessPermissions) AllowList() bool {
	return !ap.disallowLists && !ap.writeonly
}

func (ap *accessPermissions) AllowDownload() bool {
	return !ap.disallowReads && !ap.writeonly
}

func (ap *accessPermissions) AllowUpload() bool {
	return !ap.disallowWrites && !ap.readonly
}
