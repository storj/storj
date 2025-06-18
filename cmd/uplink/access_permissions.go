// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"strconv"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/storj/cmd/uplink/internal"
	"storj.io/storj/cmd/uplink/ulloc"
	"storj.io/uplink"
)

// accessPermissions holds flags and provides a Setup method for commands that
// have to modify permissions on access grants.
type accessPermissions struct {
	prefixes []uplink.SharePrefix // prefixes is the set of path prefixes that the grant will be limited to

	readonly  bool
	writeonly bool

	disallowDeletes *bool
	disallowLists   *bool
	disallowReads   *bool
	disallowWrites  *bool

	notBefore *time.Time
	notAfter  *time.Time

	maxObjectTTL *time.Duration
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

	params.Break()

	ap.disallowDeletes = params.Flag("disallow-deletes", "Disallow deletes with the access", nil,
		clingy.Transform(strconv.ParseBool), clingy.Boolean, clingy.Optional).(*bool)
	ap.disallowLists = params.Flag("disallow-lists", "Disallow lists with the access", nil,
		clingy.Transform(strconv.ParseBool), clingy.Boolean, clingy.Optional).(*bool)
	ap.disallowReads = params.Flag("disallow-reads", "Disallow reads with the access", nil,
		clingy.Transform(strconv.ParseBool), clingy.Boolean, clingy.Optional).(*bool)
	ap.disallowWrites = params.Flag("disallow-writes", "Disallow writes with the access", nil,
		clingy.Transform(strconv.ParseBool), clingy.Boolean, clingy.Optional).(*bool)

	params.Break()

	ap.notBefore = params.Flag("not-before",
		"Disallow access before this time (e.g. '+2h', 'now', '2020-01-02T15:04:05Z0700', 'none')",
		nil, clingy.Transform(internal.ParseHumanDateNotBefore), clingy.Type("relative_date"), clingy.Optional).(*time.Time)
	ap.notAfter = params.Flag("not-after",
		"Disallow access after this time (e.g. '+2h', 'now', '2020-01-02T15:04:05Z0700', 'none')",
		nil, clingy.Transform(internal.ParseHumanDateNotAfter), clingy.Type("relative_date"), clingy.Optional).(*time.Time)

	params.Break()

	ap.maxObjectTTL = params.Flag("max-object-ttl",
		"The object is automatically deleted after this period. (e.g. '1h30m', '24h', '720h')",
		nil, clingy.Transform(time.ParseDuration), clingy.Type("period"), clingy.Optional).(*time.Duration)

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
		NotBefore:     ap.NotBefore(),
		NotAfter:      ap.NotAfter(),
		MaxObjectTTL:  ap.MaxObjectTTL(),
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

func defaulted[T any](val *T, def T) T {
	if val != nil {
		return *val
	}
	return def
}

func (ap *accessPermissions) NotBefore() time.Time         { return defaulted(ap.notBefore, time.Time{}) }
func (ap *accessPermissions) NotAfter() time.Time          { return defaulted(ap.notAfter, time.Time{}) }
func (ap *accessPermissions) AllowDelete() bool            { return !defaulted(ap.disallowDeletes, ap.readonly) }
func (ap *accessPermissions) AllowList() bool              { return !defaulted(ap.disallowLists, ap.writeonly) }
func (ap *accessPermissions) AllowDownload() bool          { return !defaulted(ap.disallowReads, ap.writeonly) }
func (ap *accessPermissions) AllowUpload() bool            { return !defaulted(ap.disallowWrites, ap.readonly) }
func (ap *accessPermissions) MaxObjectTTL() *time.Duration { return ap.maxObjectTTL }
