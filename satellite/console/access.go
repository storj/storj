// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"storj.io/common/encryption"
	"storj.io/common/grant"
	"storj.io/common/macaroon"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/api"
)

// AccessPermissions holds the uplink permissions to apply to the returned access.
type AccessPermissions struct {
	AllowDownload                           bool `json:"allowDownload"`
	AllowUpload                             bool `json:"allowUpload"`
	AllowList                               bool `json:"allowList"`
	AllowDelete                             bool `json:"allowDelete"`
	AllowPutObjectRetention                 bool `json:"allowPutObjectRetention,omitempty"`
	AllowGetObjectRetention                 bool `json:"allowGetObjectRetention,omitempty"`
	AllowBypassGovernanceRetention          bool `json:"allowBypassGovernanceRetention,omitempty"`
	AllowPutObjectLegalHold                 bool `json:"allowPutObjectLegalHold,omitempty"`
	AllowGetObjectLegalHold                 bool `json:"allowGetObjectLegalHold,omitempty"`
	AllowPutBucketObjectLockConfiguration   bool `json:"allowPutBucketObjectLockConfiguration,omitempty"`
	AllowGetBucketObjectLockConfiguration   bool `json:"allowGetBucketObjectLockConfiguration,omitempty"`
	AllowPutBucketNotificationConfiguration bool `json:"allowPutBucketNotificationConfiguration,omitempty"`
	AllowGetBucketNotificationConfiguration bool `json:"allowGetBucketNotificationConfiguration,omitempty"`
}

// CreateAccessRequest is the payload for creating a restricted access grant.
type CreateAccessRequest struct {
	ProjectID   uuid.UUID         `json:"projectID"`
	Name        string            `json:"name"`
	Permissions AccessPermissions `json:"permissions"`
	Buckets     []string          `json:"buckets,omitempty"`
	NotBefore   *time.Time        `json:"notBefore,omitempty"`
	NotAfter    *time.Time        `json:"notAfter,omitempty"`
	// Passphrase is required unless the project uses satellite-managed
	// encryption, in which case it must be empty.
	Passphrase string `json:"passphrase,omitempty"`
}

// CreateAccessResponse is the result of creating a restricted access grant.
type CreateAccessResponse struct {
	Name        string `json:"name"`
	AccessGrant string `json:"accessGrant"`
}

// PrivateGenCreateAccess is the handles creation of restricted access for the private generated API.
func (s *Service) PrivateGenCreateAccess(ctx context.Context, authUser *User, req CreateAccessRequest) (_ *CreateAccessResponse, httpErr api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	s.auditLog(ctx, "create restricted access",
		&authUser.ID, authUser.Email,
		zap.Stringer("project_id", req.ProjectID),
		zap.String("name", req.Name))

	resp, err := s.createRestrictedAccess(ctx, authUser, req)
	if err != nil {
		return nil, mapAccessErrorToHTTP(err)
	}
	return resp, api.HTTPError{}
}

func (s *Service) createRestrictedAccess(ctx context.Context, user *User, req CreateAccessRequest) (_ *CreateAccessResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	if !s.config.AccessCreationHttpApiEnabled {
		return nil, ErrForbidden.New("This endpoint is not enabled")
	}

	if strings.TrimSpace(req.Name) == "" {
		return nil, ErrValidation.New("name is required")
	}
	if req.NotBefore != nil && req.NotAfter != nil && req.NotAfter.Before(*req.NotBefore) {
		return nil, ErrValidation.New("notAfter must be after notBefore")
	}

	isMember, err := s.isProjectMember(ctx, user.ID, req.ProjectID)
	if err != nil {
		return nil, ErrUnauthorized.Wrap(err)
	}
	project := isMember.project

	// Resolve passphrase: managed-encryption projects supply their own via KMS and
	// reject a user-provided one; non-managed projects require one from the caller.
	var passphrase []byte
	if project.PassphraseEnc != nil {
		if req.Passphrase != "" {
			return nil, ErrValidation.New("project uses satellite-managed encryption; custom passphrase is not allowed")
		}
		if s.kmsService == nil {
			return nil, Error.New("satellite-managed encryption is not configured on this satellite")
		}
		if project.PassphraseEncKeyID == nil {
			return nil, Error.New("failed to get passphrase")
		}
		passphrase, err = s.kmsService.DecryptPassphrase(ctx, *project.PassphraseEncKeyID, project.PassphraseEnc)
		if err != nil {
			return nil, Error.Wrap(err)
		}
	} else {
		if req.Passphrase == "" {
			return nil, ErrValidation.New("passphrase is required")
		}
		passphrase = []byte(req.Passphrase)
	}

	_, rawKey, err := s.createAPIKey(ctx, project, req.Name, user.UserAgent, user.ID)
	if err != nil {
		return nil, err
	}
	restrictedKey, err := rawKey.Restrict(permissionsToCaveat(req))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	salt, err := s.store.Projects().GetSalt(ctx, project.ID)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	pathCipher := storj.EncAESGCM
	if project.PathEncryption != nil && !*project.PathEncryption {
		pathCipher = storj.EncNull
	}

	// https://github.com/storj/storj/blob/804b9f4b99ca03f675654245447caec2660e8edc/web/satellite/wasm/consolewasm/access.go#L48
	const concurrency = 8
	rootKey, err := encryption.DeriveRootKey(passphrase, salt, "", concurrency)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	encAccess := grant.NewEncryptionAccessWithDefaultKey(rootKey)
	encAccess.SetDefaultPathCipher(pathCipher)
	encAccess.LimitTo(restrictedKey)

	serialized, err := (&grant.Access{
		SatelliteAddress: s.satelliteNodeURL,
		APIKey:           restrictedKey,
		EncAccess:        encAccess,
	}).Serialize()
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return &CreateAccessResponse{Name: req.Name, AccessGrant: serialized}, nil
}

// permissionsToCaveat converts the request permissions into a macaroon caveat.
func permissionsToCaveat(req CreateAccessRequest) macaroon.Caveat {
	p := req.Permissions

	caveat := macaroon.Caveat{
		DisallowReads:                              !p.AllowDownload,
		DisallowWrites:                             !p.AllowUpload,
		DisallowLists:                              !p.AllowList,
		DisallowDeletes:                            !p.AllowDelete,
		DisallowPutRetention:                       !p.AllowPutObjectRetention,
		DisallowGetRetention:                       !p.AllowGetObjectRetention,
		DisallowBypassGovernanceRetention:          !p.AllowBypassGovernanceRetention,
		DisallowPutLegalHold:                       !p.AllowPutObjectLegalHold,
		DisallowGetLegalHold:                       !p.AllowGetObjectLegalHold,
		DisallowPutBucketObjectLockConfiguration:   !p.AllowPutBucketObjectLockConfiguration,
		DisallowGetBucketObjectLockConfiguration:   !p.AllowGetBucketObjectLockConfiguration,
		DisallowPutBucketNotificationConfiguration: !p.AllowPutBucketNotificationConfiguration,
		DisallowGetBucketNotificationConfiguration: !p.AllowGetBucketNotificationConfiguration,
		NotBefore: req.NotBefore,
		NotAfter:  req.NotAfter,
	}

	for _, b := range req.Buckets {
		if b == "" {
			continue
		}
		caveat.AllowedPaths = append(caveat.AllowedPaths, &macaroon.Caveat_Path{Bucket: []byte(b)})
	}

	return macaroon.WithNonce(caveat)
}

// mapAccessErrorToHTTP maps errors from createRestrictedAccess to HTTP status codes.
func mapAccessErrorToHTTP(err error) api.HTTPError {
	switch {
	case ErrValidation.Has(err):
		return api.HTTPError{Status: http.StatusBadRequest, Err: Error.Wrap(err)}
	case ErrConflict.Has(err):
		return api.HTTPError{Status: http.StatusConflict, Err: Error.Wrap(err)}
	case ErrUnauthorized.Has(err), ErrNoMembership.Has(err):
		return api.HTTPError{Status: http.StatusUnauthorized, Err: Error.Wrap(err)}
	case ErrForbidden.Has(err):
		return api.HTTPError{Status: http.StatusForbidden, Err: Error.Wrap(err)}
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return api.HTTPError{Status: http.StatusRequestTimeout, Err: Error.Wrap(err)}
	default:
		return api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
	}
}
