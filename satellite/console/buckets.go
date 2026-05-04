// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"storj.io/common/errs2"
	"storj.io/common/grant"
	"storj.io/common/macaroon"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/api"
	"storj.io/uplink"
	"storj.io/uplink/private/bucket"
	"storj.io/uplink/private/metaclient"
)

// CreateBucketRequest is the payload for the public API's create bucket endpoint.
type CreateBucketRequest struct {
	ProjectID         uuid.UUID               `json:"projectID"`
	Name              string                  `json:"name"`
	Placement         string                  `json:"placement,omitempty"`
	ObjectLockEnabled bool                    `json:"objectLockEnabled,omitempty"`
	Versioning        bool                    `json:"versioning,omitempty"`
	DefaultRetention  *DefaultRetentionConfig `json:"defaultRetention,omitempty"`
}

// DefaultRetentionConfig is the default object retention configuration
// the bucket being created.
type DefaultRetentionConfig struct {
	Mode  string `json:"mode"`
	Days  int32  `json:"days,omitempty"`
	Years int32  `json:"years,omitempty"`
}

// CreateBucketResponse is returned by the public API's create bucket endpoint.
type CreateBucketResponse struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	Placement string    `json:"placement,omitempty"`
}

// GenCreateBucket creates a new bucket via the public generated API.
func (s *Service) GenCreateBucket(ctx context.Context, req CreateBucketRequest) (_ *CreateBucketResponse, httpErr api.HTTPError) {
	var err error
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "gen create bucket",
		zap.Stringer("project_id", req.ProjectID),
		zap.String("bucket", req.Name))
	if err != nil {
		return nil, api.HTTPError{Status: http.StatusUnauthorized, Err: Error.Wrap(err)}
	}

	isMember, err := s.isProjectMember(ctx, user.ID, req.ProjectID)
	if err != nil {
		return nil, api.HTTPError{Status: http.StatusUnauthorized, Err: Error.Wrap(err)}
	}

	keyName := fmt.Sprintf("public-api-bucket-create-%d", time.Now().UnixNano())
	apiKeyInfo, rawKey, apiHTTPErr := s.genCreateAPIKey(ctx, isMember.project, keyName, user.UserAgent, user.ID)
	if apiHTTPErr.Err != nil {
		return nil, apiHTTPErr
	}
	defer func() {
		if delErr := s.store.APIKeys().Delete(ctx, apiKeyInfo.ID); delErr != nil {
			s.log.Warn("failed to delete bucket-create api key",
				zap.Stringer("id", apiKeyInfo.ID), zap.Error(delErr))
		}
	}()

	parsedKey, err := macaroon.ParseAPIKey(rawKey)
	if err != nil {
		return nil, api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
	}

	accessGrant := &grant.Access{
		SatelliteAddress: s.satelliteNodeURL,
		APIKey:           parsedKey,
		EncAccess:        grant.NewEncryptionAccess(),
	}
	serialized, err := accessGrant.Serialize()
	if err != nil {
		return nil, api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
	}

	access, err := uplink.ParseAccess(serialized)
	if err != nil {
		return nil, api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
	}

	project, err := uplink.OpenProject(ctx, access)
	if err != nil {
		return nil, mapUplinkErrorToHTTP(err)
	}
	defer func() {
		closeErr := project.Close()
		if closeErr != nil {
			s.log.Warn("failed to close uplink project",
				zap.Stringer("id", apiKeyInfo.ID), zap.Error(closeErr))
		}
	}()

	created, err := bucket.CreateBucketWithObjectLock(ctx, project, bucket.CreateBucketWithObjectLockParams{
		Name:              req.Name,
		ObjectLockEnabled: req.ObjectLockEnabled,
		Placement:         req.Placement,
	})
	if err != nil {
		return nil, mapUplinkErrorToHTTP(err)
	}

	// If ObjectLockEnabled is true, CreateBucketWithObjectLock will automatically
	// enable versioning.
	if req.Versioning && !req.ObjectLockEnabled {
		if err = bucket.SetBucketVersioning(ctx, project, req.Name, true); err != nil {
			return nil, mapUplinkErrorToHTTP(err)
		}
	}

	if req.ObjectLockEnabled && req.DefaultRetention != nil {
		cfg, buildErr := buildLockConfig(req.DefaultRetention)
		if buildErr.Err != nil {
			return nil, buildErr
		}
		if err = bucket.SetBucketObjectLockConfiguration(ctx, project, req.Name, cfg); err != nil {
			return nil, mapUplinkErrorToHTTP(err)
		}
	}

	return &CreateBucketResponse{
		Name:      created.Name,
		CreatedAt: created.Created,
		Placement: req.Placement,
	}, api.HTTPError{}
}

func buildLockConfig(r *DefaultRetentionConfig) (*metaclient.BucketObjectLockConfiguration, api.HTTPError) {
	var mode storj.RetentionMode
	switch strings.ToUpper(r.Mode) {
	case "COMPLIANCE":
		mode = storj.ComplianceMode
	case "GOVERNANCE":
		mode = storj.GovernanceMode
	default:
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    ErrValidation.New("invalid default retention mode %q", r.Mode),
		}
	}
	if r.Days != 0 && r.Years != 0 {
		return nil, api.HTTPError{
			Status: http.StatusBadRequest,
			Err:    ErrValidation.New("default retention cannot specify both days and years"),
		}
	}
	return &metaclient.BucketObjectLockConfiguration{
		Enabled: true,
		DefaultRetention: &metaclient.DefaultRetention{
			Mode:  mode,
			Days:  r.Days,
			Years: r.Years,
		},
	}, api.HTTPError{}
}

func mapUplinkErrorToHTTP(err error) api.HTTPError {
	switch {
	case errors.Is(err, uplink.ErrBucketAlreadyExists):
		return api.HTTPError{Status: http.StatusConflict, Err: Error.Wrap(err)}
	case errors.Is(err, uplink.ErrBucketNameInvalid), errs2.IsRPC(err, rpcstatus.InvalidArgument):
		return api.HTTPError{Status: http.StatusBadRequest, Err: Error.Wrap(err)}
	case errors.Is(err, uplink.ErrPermissionDenied), errs2.IsRPC(err, rpcstatus.PermissionDenied):
		return api.HTTPError{Status: http.StatusForbidden, Err: Error.Wrap(err)}
	case errors.Is(err, uplink.ErrTooManyRequests):
		return api.HTTPError{Status: http.StatusTooManyRequests, Err: Error.Wrap(err)}
	case errors.Is(err, bucket.ErrBucketInvalidObjectLockConfig),
		errors.Is(err, bucket.ErrBucketInvalidStateObjectLock),
		errors.Is(err, bucket.ErrBucketNoLock):
		return api.HTTPError{Status: http.StatusPreconditionFailed, Err: Error.Wrap(err)}
	default:
		return api.HTTPError{Status: http.StatusInternalServerError, Err: Error.Wrap(err)}
	}
}
