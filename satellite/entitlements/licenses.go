// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package entitlements

import (
	"context"
	"encoding/json"
	"time"

	"storj.io/common/uuid"
)

// LicenseScopePrefix is the prefix used for license scopes in the database.
const LicenseScopePrefix = "license_scope:"

// OMLicenseType is the constant representing the Object Mount license type.
const OMLicenseType = "OM"

// AccountLicense represents a single license assigned to a user.
type AccountLicense struct {
	Type       string `json:"type,omitempty"`
	ProductID  uint   `json:"product_id,omitempty"`
	Count      int    `json:"count,omitempty"`
	PublicID   string `json:"public_id,omitempty"`
	BucketName string `json:"bucket_name,omitempty"`
	// StartsAt is when the license took effect. It is used to prorate the first
	// billing period. Licenses stored before this field was introduced have a zero
	// value and are billed for the whole period.
	StartsAt  time.Time `json:"starts_at,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	RevokedAt time.Time `json:"revoked_at,omitempty"`
	Key       []byte    `json:"key,omitempty"`
}

// UnmarshalJSON implements json.Unmarshaler. It defaults Count to 1 when absent
// so that licenses stored before the Count field was introduced are treated as
// granting a single seat.
func (al *AccountLicense) UnmarshalJSON(data []byte) error {
	type Alias AccountLicense
	if err := json.Unmarshal(data, (*Alias)(al)); err != nil {
		return err
	}
	if al.Count == 0 {
		al.Count = 1
	}
	return nil
}

// AccountLicenses represents a collection of licenses assigned to a user.
type AccountLicenses struct {
	Licenses []AccountLicense `json:"licenses,omitempty"`
}

// Clone creates a deep copy of the AccountLicenses instance.
func (al *AccountLicenses) Clone() AccountLicenses {
	clone := AccountLicenses{
		Licenses: make([]AccountLicense, len(al.Licenses)),
	}
	copy(clone.Licenses, al.Licenses)

	return clone
}

// Licenses separates license-related entitlements functionality.
type Licenses struct {
	service *Service
}

// Get retrieves the licenses of a user by their user ID.
func (p *Licenses) Get(ctx context.Context, userID uuid.UUID) (licenses AccountLicenses, err error) {
	defer mon.Task()(&ctx)(&err)

	ent, err := p.service.db.GetByScope(ctx, ConvertUserIDToLicenseScope(userID))
	if err != nil {
		if ErrNotFound.Has(err) {
			return AccountLicenses{}, nil
		}
		return AccountLicenses{}, Error.Wrap(err)
	}

	err = json.Unmarshal(ent.Features, &licenses)
	return licenses, Error.Wrap(err)
}

// GetActiveOptions defines the options for retrieving active licenses.
type GetActiveOptions struct {
	PublicID    uuid.UUID
	BucketName  string
	LicenseType string

	Now *time.Time
}

// GetActive retrieves the active licenses of a user by their user ID, filtered by the provided options.
func (p *Licenses) GetActive(ctx context.Context, userID uuid.UUID, opts GetActiveOptions) (_ []AccountLicense, err error) {
	defer mon.Task()(&ctx)(&err)

	ent, err := p.service.db.GetByScope(ctx, ConvertUserIDToLicenseScope(userID))
	if err != nil {
		if ErrNotFound.Has(err) {
			return nil, nil
		}
		return nil, Error.Wrap(err)
	}

	var licenses AccountLicenses
	if err := json.Unmarshal(ent.Features, &licenses); err != nil {
		return nil, Error.Wrap(err)
	}

	var result []AccountLicense
	for _, license := range licenses.Licenses {
		// Filter by license type if specified
		if opts.LicenseType != "" && license.Type != opts.LicenseType {
			continue
		}

		// Filter by time if specified - skip licenses that are not in force yet, or
		// that are expired or revoked. A zero StartsAt is a license from before the
		// field existed, so it is treated as always having been in force, matching
		// BillableSeatDays.
		if opts.Now != nil &&
			((!license.StartsAt.IsZero() && license.StartsAt.After(*opts.Now)) ||
				(!license.ExpiresAt.IsZero() && !license.ExpiresAt.After(*opts.Now)) ||
				(!license.RevokedAt.IsZero() && !license.RevokedAt.After(*opts.Now))) {
			continue
		}

		// Filter by project and bucket (empty values act as wildcards)
		projectMatches := license.PublicID == "" || opts.PublicID.IsZero() || license.PublicID == opts.PublicID.String()
		bucketMatches := license.BucketName == "" || opts.BucketName == "" || license.BucketName == opts.BucketName

		if projectMatches && bucketMatches {
			result = append(result, license)
		}
	}

	return result, nil
}

// earliest returns the earliest of the given times, ignoring unset ones. It returns
// the zero time if all of them are unset.
func earliest(xs ...time.Time) time.Time {
	var result time.Time
	for _, x := range xs {
		if x.IsZero() {
			continue
		}
		if result.IsZero() || x.Before(result) {
			result = x
		}
	}

	return result
}

// latest returns the latest of the given times, ignoring unset ones. It returns the
// zero time if all of them are unset.
func latest(xs ...time.Time) time.Time {
	var result time.Time
	for _, x := range xs {
		if x.IsZero() {
			continue
		}
		if result.IsZero() || x.After(result) {
			result = x
		}
	}

	return result
}

// BillableSeatDays reports how many days of the period [periodStart, periodEnd) the
// license accrues seat charges for, alongside the total length of the period in days.
// ok is false when the license is not billable in that period at all.
//
// A license is billed for the days it is actually in force: prorated from StartsAt when
// it started mid-period, and capped at whichever of ExpiresAt or RevokedAt ends it
// mid-period. A license revoked or expired at or before periodStart is not billable at
// all, and neither is one that starts and ends within the same day.
func (al *AccountLicense) BillableSeatDays(periodStart, periodEnd time.Time) (billedDays, daysInPeriod int, ok bool) {
	daysInPeriod = daysBetween(periodStart, periodEnd)
	if daysInPeriod <= 0 {
		return 0, 0, false
	}

	billedFrom := latest(periodStart, al.StartsAt)
	// The charge stops when the license does, at the earlier of its expiry and its
	// revocation.
	billedTo := earliest(periodEnd, al.ExpiresAt, al.RevokedAt)

	billedDays = daysBetween(billedFrom, billedTo)
	if billedDays <= 0 {
		return 0, daysInPeriod, false
	}

	return min(billedDays, daysInPeriod), daysInPeriod, true
}

// daysBetween returns the whole UTC days between from and to. Both ends are truncated
// to midnight, so licenses that follow one another tile a period exactly: rounding the
// duration up instead would charge the day they change hands to both of them.
func daysBetween(from, to time.Time) int {
	const day = 24 * time.Hour

	days := int(to.UTC().Truncate(day).Sub(from.UTC().Truncate(day)) / day)
	if days < 0 {
		return 0
	}
	return days
}

// Set sets the licenses for a user by their user ID.
func (p *Licenses) Set(ctx context.Context, userID uuid.UUID, licenses AccountLicenses) (err error) {
	defer mon.Task()(&ctx)(&err)

	scope := ConvertUserIDToLicenseScope(userID)

	ent, err := getEntitlementBeforeSet(ctx, p.service.db, scope)
	if err != nil {
		return Error.Wrap(err)
	}

	return Error.Wrap(upsertNewEntitlement(ctx, p.service.db, ent, licenses))
}

// ConvertUserIDToLicenseScope converts a public user ID to a database license scope.
func ConvertUserIDToLicenseScope(userID uuid.UUID) []byte {
	return append([]byte(LicenseScopePrefix), userID[:]...)
}
