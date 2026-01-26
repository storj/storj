// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"database/sql"
	"errors"
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
	Type       string     `json:"type"`
	PublicId   string     `json:"publicId,omitempty"`
	BucketName string     `json:"bucketName,omitempty"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
	Key        string     `json:"key,omitempty"`
}

// UserLicensesResponse represents the list of licenses for a user.
type UserLicensesResponse struct {
	Licenses []UserLicense `json:"licenses"`
}

// GrantLicenseRequest represents a request to grant a license to a user.
type GrantLicenseRequest struct {
	Type       string    `json:"type"`
	PublicId   string    `json:"publicId,omitempty"`
	BucketName string    `json:"bucketName,omitempty"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Key        string    `json:"key,omitempty"`
	Reason     string    `json:"reason"`
}

// RevokeLicenseRequest represents a request to revoke a license.
type RevokeLicenseRequest struct {
	Type       string    `json:"type"`
	PublicId   string    `json:"publicId,omitempty"`
	BucketName string    `json:"bucketName,omitempty"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Reason     string    `json:"reason"`
}

// DeleteLicenseRequest represents a request to permanently delete a license.
type DeleteLicenseRequest struct {
	Type       string    `json:"type"`
	PublicId   string    `json:"publicId,omitempty"`
	BucketName string    `json:"bucketName,omitempty"`
	ExpiresAt  time.Time `json:"expiresAt"`
	Reason     string    `json:"reason"`
}

// GetUserLicenses returns all licenses for a user by their ID.
func (s *Service) GetUserLicenses(ctx context.Context, userID uuid.UUID) (*UserLicensesResponse, api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

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
		result.Licenses = append(result.Licenses, UserLicense{
			Type:       license.Type,
			PublicId:   license.PublicID,
			BucketName: license.BucketName,
			ExpiresAt:  license.ExpiresAt,
			RevokedAt:  revokedAt,
			Key:        string(license.Key),
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

	if authInfo == nil || len(authInfo.Groups) == 0 {
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

	user, err := s.consoleDB.Users().Get(ctx, userID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
			err = errs.New("user not found")
		}
		return apiError(status, err)
	}

	// Validate public ID if provided
	if request.PublicId != "" {
		publicID, err := uuid.FromString(request.PublicId)
		if err != nil {
			return apiError(http.StatusBadRequest, errs.New("invalid public ID format"))
		}
		_, err = s.consoleDB.Projects().GetByPublicID(ctx, publicID)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, sql.ErrNoRows) {
				status = http.StatusNotFound
				err = errs.New("project not found")
			}
			return apiError(status, err)
		}
	}

	// Get current licenses
	currentLicenses, err := s.entitlements.Licenses().Get(ctx, user.ID)
	if err != nil {
		return apiError(http.StatusInternalServerError, err)
	}

	// Check if active license with same type and scope already exists
	for _, license := range currentLicenses.Licenses {
		if license.Type == request.Type &&
			license.PublicID == request.PublicId &&
			license.BucketName == request.BucketName &&
			license.RevokedAt.IsZero() && license.ExpiresAt.After(s.nowFn()) {
			return apiError(http.StatusConflict, errs.New("license with same type and scope already exists"))
		}
	}

	beforeState := currentLicenses

	// Add new license
	newLicense := entitlements.AccountLicense{
		Type:       request.Type,
		PublicID:   request.PublicId,
		BucketName: request.BucketName,
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

	if authInfo == nil || len(authInfo.Groups) == 0 {
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

	beforeState := currentLicenses

	// Find and revoke the license matching all fields
	found := false
	now := s.nowFn()
	for i, license := range currentLicenses.Licenses {
		if license.Type == request.Type &&
			license.PublicID == request.PublicId &&
			license.BucketName == request.BucketName &&
			license.ExpiresAt.Equal(request.ExpiresAt) {
			currentLicenses.Licenses[i].RevokedAt = now
			found = true
			break
		}
	}

	if !found {
		return apiError(http.StatusNotFound, errs.New("license not found"))
	}

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

	if authInfo == nil || len(authInfo.Groups) == 0 {
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

	beforeState := currentLicenses

	// Find and remove the license matching all fields
	found := false
	for i, license := range currentLicenses.Licenses {
		if license.Type == request.Type &&
			license.PublicID == request.PublicId &&
			license.BucketName == request.BucketName &&
			license.ExpiresAt.Equal(request.ExpiresAt) {
			currentLicenses.Licenses = append(currentLicenses.Licenses[:i], currentLicenses.Licenses[i+1:]...)
			found = true
			break
		}
	}

	if !found {
		return apiError(http.StatusNotFound, errs.New("license not found"))
	}

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
