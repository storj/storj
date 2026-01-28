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
