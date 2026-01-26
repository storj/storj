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

// AccountLicense represents a single license assigned to a user.
type AccountLicense struct {
	Type       string    `json:"type,omitempty"`
	PublicID   string    `json:"public_id,omitempty"`
	BucketName string    `json:"bucket_name,omitempty"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
	RevokedAt  time.Time `json:"revoked_at,omitempty"`
}

// AccountLicenses represents a collection of licenses assigned to a user.
type AccountLicenses struct {
	Licenses []AccountLicense `json:"licenses,omitempty"`
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

		// Filter by time if specified - skip expired or revoked licenses
		if opts.Now != nil &&
			((!license.ExpiresAt.IsZero() && !license.ExpiresAt.After(*opts.Now)) ||
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
