// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package consolewasm

import (
	"errors"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/macaroon"
)

// Permission defines what actions can be used to share.
//
// This struct has been taken from storj.io/uplink and duplicated to avoid
// pulling in that dependency.
type Permission struct {
	// AllowDownload gives permission to download the object's content. It
	// allows getting object metadata, but it does not allow listing buckets.
	AllowDownload bool
	// AllowUpload gives permission to create buckets and upload new objects.
	// It does not allow overwriting existing objects unless AllowDelete is
	// granted too.
	AllowUpload bool
	// AllowList gives permission to list buckets. It allows getting object
	// metadata, but it does not allow downloading the object's content.
	AllowList bool
	// AllowDelete gives permission to delete buckets and objects. Unless
	// either AllowDownload or AllowList is granted too, no object metadata and
	// no error info will be returned for deleted objects.
	AllowDelete bool
	// AllowPutObjectRetention gives permission for retention periods to be
	// placed on and retrieved from objects.
	AllowPutObjectRetention bool
	// AllowGetObjectRetention gives permission for retention periods to be
	// retrieved from objects.
	AllowGetObjectRetention bool
	// AllowPutObjectLegalHold gives permission for legal hold status to be
	// placed on objects.
	AllowPutObjectLegalHold bool
	// AllowGetObjectLegalHold gives permission for legal hold status to be
	// retrieved from objects.
	AllowGetObjectLegalHold bool
	// AllowBypassGovernanceRetention gives permission for governance retention
	// to be bypassed on objects.
	AllowBypassGovernanceRetention bool
	// AllowPutBucketObjectLockConfiguration gives permission for default retention config to be
	// placed on buckets.
	AllowPutBucketObjectLockConfiguration bool
	// AllowGetBucketObjectLockConfiguration gives permission for default retention config to be
	// retrieved from buckets.
	AllowGetBucketObjectLockConfiguration bool
	// NotBefore restricts when the resulting access grant is valid for.
	// If set, the resulting access grant will not work if the Satellite
	// believes the time is before NotBefore.
	// If set, this value should always be before NotAfter.
	NotBefore time.Time
	// NotAfter restricts when the resulting access grant is valid for.
	// If set, the resulting access grant will not work if the Satellite
	// believes the time is after NotAfter.
	// If set, this value should always be after NotBefore.
	NotAfter time.Time
	// MaxObjectTTL restricts the maximum time-to-live of objects.
	// If set, new objects are uploaded with an expiration time that reflects
	// the MaxObjectTTL period.
	// If objects are uploaded with an explicit expiration time, the upload
	// will be successful only if it is shorter than the MaxObjectTTL period.
	MaxObjectTTL *time.Duration
}

// SetPermission restricts the api key with the permissions and returns an api key with restricted permissions.
func SetPermission(key string, buckets []string, permission Permission) (*macaroon.APIKey, error) {
	if permission == (Permission{}) {
		return nil, errs.New("permission is empty")
	}

	var notBefore, notAfter *time.Time
	if !permission.NotBefore.IsZero() {
		notBefore = &permission.NotBefore
	}
	if !permission.NotAfter.IsZero() {
		notAfter = &permission.NotAfter
	}

	if notBefore != nil && notAfter != nil && notAfter.Before(*notBefore) {
		return nil, errs.New("invalid time range")
	}

	if permission.MaxObjectTTL != nil && *(permission.MaxObjectTTL) <= 0 {
		return nil, errors.New("non-positive ttl period")
	}

	caveat := macaroon.WithNonce(macaroon.Caveat{
		DisallowReads:                            !permission.AllowDownload,
		DisallowWrites:                           !permission.AllowUpload,
		DisallowLists:                            !permission.AllowList,
		DisallowDeletes:                          !permission.AllowDelete,
		DisallowPutRetention:                     !permission.AllowPutObjectRetention,
		DisallowGetRetention:                     !permission.AllowGetObjectRetention,
		DisallowBypassGovernanceRetention:        !permission.AllowBypassGovernanceRetention,
		DisallowPutLegalHold:                     !permission.AllowPutObjectLegalHold,
		DisallowGetLegalHold:                     !permission.AllowGetObjectLegalHold,
		DisallowPutBucketObjectLockConfiguration: !permission.AllowPutBucketObjectLockConfiguration,
		DisallowGetBucketObjectLockConfiguration: !permission.AllowGetBucketObjectLockConfiguration,
		NotBefore:                                notBefore,
		NotAfter:                                 notAfter,
		MaxObjectTtl:                             permission.MaxObjectTTL,
	})

	for _, b := range buckets {
		caveat.AllowedPaths = append(caveat.AllowedPaths, &macaroon.Caveat_Path{
			Bucket: []byte(b),
		})
	}

	apiKey, err := macaroon.ParseAPIKey(key)
	if err != nil {
		return nil, err
	}

	restrictedKey, err := apiKey.Restrict(caveat)
	if err != nil {
		return nil, err
	}

	return restrictedKey, nil
}
