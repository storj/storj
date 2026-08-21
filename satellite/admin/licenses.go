// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/storj/satellite/admin/auditlogger"
	"storj.io/storj/satellite/admin/changehistory"
	"storj.io/storj/satellite/entitlements"
)

// UserLicense represents a license assigned to a user.
type UserLicense struct {
	Type        string `json:"type"`
	ProductID   uint   `json:"productId,omitempty"`
	ProductName string `json:"productName,omitempty"`
	Count       int    `json:"count"`
	PublicId    string `json:"publicId,omitempty"`
	BucketName  string `json:"bucketName,omitempty"`
	// StartsAt is when the license took effect and so what its first billing period
	// is prorated from. It is zero for licenses granted before it was recorded.
	StartsAt  *time.Time `json:"startsAt,omitempty"`
	ExpiresAt time.Time  `json:"expiresAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
	Key       string     `json:"key,omitempty"`
}

// UserLicensesResponse represents the list of licenses for a user.
type UserLicensesResponse struct {
	Licenses []UserLicense `json:"licenses"`
}

// GrantLicenseRequest represents a request to grant a license to a user.
type GrantLicenseRequest struct {
	Type       string    `json:"type"`
	ProductID  uint      `json:"productId,omitempty"`
	Count      int       `json:"count,omitempty"`
	PublicId   string    `json:"publicId,omitempty"`
	BucketName string    `json:"bucketName,omitempty"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Key        string    `json:"key,omitempty"`
	Reason     string    `json:"reason"`
}

// RevokeLicenseRequest represents a request to revoke a license.
type RevokeLicenseRequest struct {
	Type       string    `json:"type"`
	ProductID  uint      `json:"productId,omitempty"`
	PublicId   string    `json:"publicId,omitempty"`
	BucketName string    `json:"bucketName,omitempty"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Reason     string    `json:"reason"`
}

// DeleteLicenseRequest represents a request to permanently delete a license.
type DeleteLicenseRequest struct {
	Type       string    `json:"type"`
	ProductID  uint      `json:"productId,omitempty"`
	PublicId   string    `json:"publicId,omitempty"`
	BucketName string    `json:"bucketName,omitempty"`
	ExpiresAt  time.Time `json:"expiresAt"`
	// RevokedAt tells a revoked license apart from an active twin of it.
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
	Reason    string     `json:"reason"`
}

// UpdateLicenseRequest represents a request to update a license's expiration time.
type UpdateLicenseRequest struct {
	Type         string    `json:"type"`
	ProductID    uint      `json:"productId,omitempty"`
	PublicId     string    `json:"publicId,omitempty"`
	BucketName   string    `json:"bucketName,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt"`
	NewExpiresAt time.Time `json:"newExpiresAt"`
	Reason       string    `json:"reason"`
}

// canonicalScope returns the spelling of a project public ID that scope matching uses,
// since uuid.FromString also accepts the undashed and uppercase forms; anything that is
// not a public ID is returned unchanged.
func canonicalScope(publicID string) string {
	parsed, err := uuid.FromString(publicID)
	if err != nil {
		return publicID
	}

	return parsed.String()
}

// scopeOverlaps reports whether two license scopes cover any of the same traffic. An
// empty value is a wildcard, as it is in entitlements.Licenses.GetActive, so an
// account-wide license covers every project and bucket that a narrower one does.
func scopeOverlaps(a, b string) bool {
	return a == "" || b == "" || a == b
}

// conflictingLicense returns a license already in force that must not coexist with one
// of the given type, product and scope. skip is the license being changed, or -1.
func conflictingLicense(licenses []entitlements.AccountLicense, licenseType string, productID uint, publicID, bucketName string, now time.Time, skip int) *entitlements.AccountLicense {
	for i, license := range licenses {
		if i == skip || license.Type != licenseType {
			continue
		}
		// A zero ExpiresAt is a perpetual license: GetActive keeps returning it and
		// BillableSeatDays keeps billing it, so it is in force here too.
		if !license.RevokedAt.IsZero() ||
			(!license.ExpiresAt.IsZero() && !license.ExpiresAt.After(now)) {
			continue
		}
		// A scope stored before grants normalized it may be spelled differently.
		scope := canonicalScope(license.PublicID)
		if !scopeOverlaps(scope, publicID) ||
			!scopeOverlaps(license.BucketName, bucketName) {
			continue
		}
		// Two paid licenses that cover any of the same traffic each bill their seats
		// for it, whether they name the same product or not, and an account-wide
		// license covers everything a narrower one does.
		if license.ProductID != 0 && productID != 0 {
			return &licenses[i]
		}
		// The same product on the very same scope is the same license: a second one
		// would be ambiguous to revoke, update and delete. Free and paid may coexist,
		// and a free license bills nothing, so nothing wider than that is refused.
		if license.ProductID == productID &&
			scope == publicID && license.BucketName == bucketName {
			return &licenses[i]
		}
	}

	return nil
}

// findLicense returns the index of the active and of the revoked license a request
// refers to, or -1 for each. A set revokedAt narrows the revoked match to that time.
func findLicense(licenses []entitlements.AccountLicense, licenseType string, productID uint, publicID, bucketName string, expiresAt, revokedAt time.Time) (active, revoked int) {
	active, revoked = -1, -1

	for i, license := range licenses {
		if license.Type != licenseType ||
			license.ProductID != productID ||
			canonicalScope(license.PublicID) != publicID ||
			license.BucketName != bucketName ||
			!license.ExpiresAt.Equal(expiresAt) {
			continue
		}

		if license.RevokedAt.IsZero() {
			if active < 0 {
				active = i
			}
		} else if revoked < 0 && (revokedAt.IsZero() || license.RevokedAt.Equal(revokedAt)) {
			revoked = i
		}
	}

	return active, revoked
}

// GetUserLicenses returns all licenses for a user by their ID.
func (s *Service) GetUserLicenses(ctx context.Context, userID uuid.UUID) (*UserLicensesResponse, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	if s.tenantID != nil {
		return nil, api.HTTPError{Status: http.StatusForbidden, Err: Error.New("not available for tenant-scoped admin")}
	}

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errors.New("user not found")
		}
		return nil, api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	licenses, err := s.entitlements.Licenses().Get(ctx, user.ID)
	if err != nil {
		return nil, api.HTTPError{
			Status: http.StatusInternalServerError,
			Err:    Error.Wrap(err),
		}
	}

	result := &UserLicensesResponse{
		Licenses: make([]UserLicense, 0, len(licenses.Licenses)),
	}

	for _, license := range licenses.Licenses {
		var revokedAt *time.Time
		if !license.RevokedAt.IsZero() {
			revokedAt = &license.RevokedAt
		}

		var startsAt *time.Time
		if !license.StartsAt.IsZero() {
			startsAt = &license.StartsAt
		}

		productName := "Free"
		if license.ProductID != 0 {
			productName = "Unknown Product"
			// Granting rejects a product ID wider than int32, but an older license may
			// hold one. Billing truncates it and charges the truncated product, so
			// naming that product here would hide the mismatch instead of showing it.
			if license.ProductID > math.MaxInt32 {
				s.log.Warn("product ID on license is out of range", zap.Uint("product_id", license.ProductID))
			} else if info, lookupErr := s.getProductByID(int32(license.ProductID)); lookupErr != nil {
				s.log.Warn("unknown product ID on license", zap.Uint("product_id", license.ProductID), zap.Error(lookupErr))
			} else {
				productName = info.ProductName
			}
		}

		result.Licenses = append(result.Licenses, UserLicense{
			Type:        license.Type,
			ProductID:   license.ProductID,
			ProductName: productName,
			Count:       license.Count,
			PublicId:    license.PublicID,
			BucketName:  license.BucketName,
			StartsAt:    startsAt,
			ExpiresAt:   license.ExpiresAt,
			RevokedAt:   revokedAt,
			Key:         string(license.Key),
		})
	}

	return result, api.HTTPError{}
}

// GrantUserLicense grants a new license to a user.
func (s *Service) GrantUserLicense(ctx context.Context, authInfo *AuthInfo, userID uuid.UUID, request GrantLicenseRequest) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) api.HTTPError {
		return api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	if s.tenantID != nil {
		return apiError(http.StatusForbidden, errs.New("not available for tenant-scoped admin"))
	}

	if !s.authorizer.IsAuthorized(authInfo) {
		return apiError(http.StatusUnauthorized, errs.New("not authorized"))
	}

	if request.Reason == "" {
		return apiError(http.StatusBadRequest, errs.New("reason is required"))
	}

	if request.Type == "" {
		return apiError(http.StatusBadRequest, errs.New("license type is required"))
	}

	if request.ExpiresAt.IsZero() {
		return apiError(http.StatusBadRequest, errs.New("expiration date is required"))
	}

	if request.ExpiresAt.Before(s.nowFn()) {
		return apiError(http.StatusBadRequest, errs.New("expiration date must be in the future"))
	}

	if request.Count <= 0 {
		return apiError(http.StatusBadRequest, errs.New("Seat count must be greater than zero"))
	}

	if request.ProductID != 0 {
		if request.ProductID > math.MaxInt32 {
			return apiError(http.StatusBadRequest, errs.New("unknown product ID %d", request.ProductID))
		}
		product, exists := s.products[int32(request.ProductID)]
		if !exists {
			return apiError(http.StatusBadRequest, errs.New("unknown product ID %d", request.ProductID))
		}
		if product.LicenseFeeCents.IsZero() {
			return apiError(http.StatusBadRequest, errs.New("product %d (%q) has no license fee", request.ProductID, product.ProductName))
		}
	}

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errs.New("user not found")
		}
		return apiError(status, err)
	}

	// GetActive matches the scope against UUID.String(), so store that spelling.
	publicID := request.PublicId
	if publicID != "" {
		parsed, err := uuid.FromString(publicID)
		if err != nil {
			return apiError(http.StatusBadRequest, errs.New("invalid public ID format"))
		}
		if _, err := s.consoleDB.Projects().GetByPublicID(ctx, parsed); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, sql.ErrNoRows) {
				status = http.StatusNotFound
				err = errs.New("project not found")
			}
			return apiError(status, err)
		}
		publicID = parsed.String()
	}

	// Get current licenses
	currentLicenses, err := s.entitlements.Licenses().Get(ctx, user.ID)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}

	if conflict := conflictingLicense(currentLicenses.Licenses, request.Type, request.ProductID,
		publicID, request.BucketName, s.nowFn(), -1); conflict != nil {
		if conflict.ProductID == request.ProductID {
			return apiError(http.StatusConflict, errs.New("license with same type and product already covers this scope"))
		}
		return apiError(http.StatusConflict, errs.New("license with same type already covers this scope for product %d", conflict.ProductID))
	}

	beforeState := currentLicenses.Clone()

	// Add new license
	newLicense := entitlements.AccountLicense{
		Type:       request.Type,
		ProductID:  request.ProductID,
		Count:      request.Count,
		PublicID:   publicID,
		BucketName: request.BucketName,
		StartsAt:   s.nowFn(),
		ExpiresAt:  request.ExpiresAt,
		Key:        []byte(request.Key),
	}
	currentLicenses.Licenses = append(currentLicenses.Licenses, newLicense)

	err = s.entitlements.Licenses().Set(ctx, user.ID, currentLicenses)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}

	afterState, err := s.entitlements.Licenses().Get(ctx, user.ID)
	if err != nil {
		s.log.Error("Failed to retrieve licenses after granting", zap.Stringer("user_id", user.ID), zap.Error(err))
	}

	s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
		UserID:     userID,
		Action:     "grant_user_license",
		AdminEmail: authInfo.Email,
		ItemType:   changehistory.ItemTypeUser,
		Reason:     request.Reason,
		Before:     beforeState,
		After:      afterState,
		Timestamp:  s.nowFn(),
	})

	return api.HTTPError{}
}

// RevokeUserLicense revokes a license for a user by setting the RevokedAt timestamp.
func (s *Service) RevokeUserLicense(ctx context.Context, authInfo *AuthInfo, userID uuid.UUID, request RevokeLicenseRequest) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) api.HTTPError {
		return api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	if s.tenantID != nil {
		return apiError(http.StatusForbidden, errs.New("not available for tenant-scoped admin"))
	}

	if !s.authorizer.IsAuthorized(authInfo) {
		return apiError(http.StatusUnauthorized, errs.New("not authorized"))
	}

	if request.Reason == "" {
		return apiError(http.StatusBadRequest, errs.New("reason is required"))
	}

	if request.Type == "" {
		return apiError(http.StatusBadRequest, errs.New("license type is required"))
	}

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errs.New("user not found")
		}
		return apiError(status, err)
	}

	// Get current licenses
	currentLicenses, err := s.entitlements.Licenses().Get(ctx, user.ID)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}

	beforeState := currentLicenses.Clone()

	active, revoked := findLicense(currentLicenses.Licenses, request.Type, request.ProductID,
		canonicalScope(request.PublicId), request.BucketName, request.ExpiresAt, time.Time{})
	if active < 0 {
		if revoked >= 0 {
			return apiError(http.StatusBadRequest, errs.New("license is already revoked"))
		}
		return apiError(http.StatusNotFound, errs.New("license not found"))
	}
	currentLicenses.Licenses[active].RevokedAt = s.nowFn()

	err = s.entitlements.Licenses().Set(ctx, user.ID, currentLicenses)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}

	afterState, err := s.entitlements.Licenses().Get(ctx, user.ID)
	if err != nil {
		s.log.Error("Failed to retrieve licenses after revoking", zap.Stringer("user_id", user.ID), zap.Error(err))
	}

	s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
		UserID:     userID,
		Action:     "revoke_user_license",
		AdminEmail: authInfo.Email,
		ItemType:   changehistory.ItemTypeUser,
		Reason:     request.Reason,
		Before:     beforeState,
		After:      afterState,
		Timestamp:  s.nowFn(),
	})

	return api.HTTPError{}
}

// DeleteUserLicense permanently removes a license from a user.
func (s *Service) DeleteUserLicense(ctx context.Context, authInfo *AuthInfo, userID uuid.UUID, request DeleteLicenseRequest) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) api.HTTPError {
		return api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	if s.tenantID != nil {
		return apiError(http.StatusForbidden, errs.New("not available for tenant-scoped admin"))
	}

	if !s.authorizer.IsAuthorized(authInfo) {
		return apiError(http.StatusUnauthorized, errs.New("not authorized"))
	}

	if request.Reason == "" {
		return apiError(http.StatusBadRequest, errs.New("reason is required"))
	}

	if request.Type == "" {
		return apiError(http.StatusBadRequest, errs.New("license type is required"))
	}

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errs.New("user not found")
		}
		return apiError(status, err)
	}

	// Get current licenses
	currentLicenses, err := s.entitlements.Licenses().Get(ctx, user.ID)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}

	beforeState := currentLicenses.Clone()

	var revokedAt time.Time
	if request.RevokedAt != nil {
		revokedAt = *request.RevokedAt
	}

	active, revoked := findLicense(currentLicenses.Licenses, request.Type, request.ProductID,
		canonicalScope(request.PublicId), request.BucketName, request.ExpiresAt, revokedAt)
	// A request naming a revocation time is for that license alone. Without one, prefer
	// the active license and fall back to a revoked one so cleanup still works.
	idx := active
	if !revokedAt.IsZero() || active < 0 {
		idx = revoked
	}
	if idx < 0 {
		return apiError(http.StatusNotFound, errs.New("license not found"))
	}
	currentLicenses.Licenses = append(currentLicenses.Licenses[:idx], currentLicenses.Licenses[idx+1:]...)

	err = s.entitlements.Licenses().Set(ctx, user.ID, currentLicenses)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}

	afterState, err := s.entitlements.Licenses().Get(ctx, user.ID)
	if err != nil {
		s.log.Error("Failed to retrieve licenses after deleting", zap.Stringer("user_id", user.ID), zap.Error(err))
	}

	s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
		UserID:     userID,
		Action:     "delete_user_license",
		AdminEmail: authInfo.Email,
		ItemType:   changehistory.ItemTypeUser,
		Reason:     request.Reason,
		Before:     beforeState,
		After:      afterState,
		Timestamp:  s.nowFn(),
	})

	return api.HTTPError{}
}

// UpdateUserLicense updates a license's expiration time for a user.
func (s *Service) UpdateUserLicense(ctx context.Context, authInfo *AuthInfo, userID uuid.UUID, request UpdateLicenseRequest) api.HTTPError {
	var err error
	defer mon.Task()(&ctx)(&err)

	apiError := func(status int, err error) api.HTTPError {
		return api.HTTPError{
			Status: status, Err: Error.Wrap(err),
		}
	}

	if s.tenantID != nil {
		return apiError(http.StatusForbidden, errs.New("not available for tenant-scoped admin"))
	}

	if !s.authorizer.IsAuthorized(authInfo) {
		return apiError(http.StatusUnauthorized, errs.New("not authorized"))
	}

	if request.Reason == "" {
		return apiError(http.StatusBadRequest, errs.New("reason is required"))
	}

	if request.Type == "" {
		return apiError(http.StatusBadRequest, errs.New("license type is required"))
	}

	if request.NewExpiresAt.IsZero() {
		return apiError(http.StatusBadRequest, errs.New("new expiration date is required"))
	}

	if request.NewExpiresAt.Before(s.nowFn()) {
		return apiError(http.StatusBadRequest, errs.New("new expiration date must be in the future"))
	}

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errs.New("user not found")
		}
		return apiError(status, err)
	}

	// Get current licenses
	currentLicenses, err := s.entitlements.Licenses().Get(ctx, user.ID)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}

	beforeState := currentLicenses.Clone()

	active, revoked := findLicense(currentLicenses.Licenses, request.Type, request.ProductID,
		canonicalScope(request.PublicId), request.BucketName, request.ExpiresAt, time.Time{})
	if active < 0 {
		if revoked >= 0 {
			return apiError(http.StatusBadRequest, errs.New("cannot update a revoked license"))
		}
		return apiError(http.StatusNotFound, errs.New("license not found"))
	}

	// Moving the expiry of a lapsed license does not renew it: StartsAt would still
	// point at the term that already ended, so BillableSeatDays would bill the whole
	// gap, including closed periods that are invoiced later. Expiry also does not block
	// a grant, so the lapsed license may already have a twin that took over from it.
	// Granting is the operation that starts a new term and prorates it from the grant.
	if !currentLicenses.Licenses[active].ExpiresAt.IsZero() &&
		!currentLicenses.Licenses[active].ExpiresAt.After(s.nowFn()) {
		return apiError(http.StatusBadRequest, errs.New("cannot extend an expired license, grant a new one instead"))
	}

	// Two licenses in force over the same scope can only predate the grant conflict
	// check, but extending one of them would prolong the overlap, so it is refused for
	// the same reason a grant would be.
	if conflict := conflictingLicense(currentLicenses.Licenses, request.Type, request.ProductID,
		canonicalScope(request.PublicId), request.BucketName, s.nowFn(), active); conflict != nil {
		return apiError(http.StatusConflict, errs.New("license with same type already in force for product %d covers this scope", conflict.ProductID))
	}

	currentLicenses.Licenses[active].ExpiresAt = request.NewExpiresAt

	err = s.entitlements.Licenses().Set(ctx, user.ID, currentLicenses)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}

	afterState, err := s.entitlements.Licenses().Get(ctx, user.ID)
	if err != nil {
		s.log.Error("Failed to retrieve licenses after updating", zap.Stringer("user_id", user.ID), zap.Error(err))
	}

	s.auditLogger.EnqueueChangeEvent(auditlogger.Event{
		UserID:     userID,
		Action:     "update_user_license",
		AdminEmail: authInfo.Email,
		ItemType:   changehistory.ItemTypeUser,
		Reason:     request.Reason,
		Before:     beforeState,
		After:      afterState,
		Timestamp:  s.nowFn(),
	})

	return api.HTTPError{}
}
