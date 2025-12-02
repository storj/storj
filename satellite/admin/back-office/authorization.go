// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"net/http"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/api"
)

// These constants are the list of permissions that the service uses for authorizing users to
// perform operations.
const (
	PermAccountView Permission = 1 << iota
	PermAccountChangeEmail
	PermAccountChangeName
	PermAccountChangeKind
	PermAccountChangeStatus
	PermAccountDisableMFA
	PermAccountChangeLimits
	PermAccountSetDataPlacement
	PermAccountRemoveDataPlacement
	PermAccountSetUserAgent
	PermAccountSuspendTemporary
	PermAccountReActivateTemporary
	PermAccountSuspendPermanently
	PermAccountReActivatePermanently
	PermAccountMarkPendingDeletion
	PermAccountDeleteNoData
	PermAccountCreateRestKey
	PermAccountDeleteWithData
	PermProjectView
	PermProjectSetLimits
	PermProjectUpdate
	PermProjectSetDataPlacement
	PermProjectSetEntitlements
	PermProjectRemoveDataPlacement
	PermProjectSetUserAgent
	PermProjectSendInvitation
	PermProjectDeleteNoData
	PermProjectMarkPendingDeletion
	PermBucketView
	PermBucketSetDataPlacement
	PermBucketRemoveDataPlacement
	PermBucketSetUserAgent
	PermViewChangeHistory
)

// These constants are the list of roles that users can have and the service uses to match
// permissions to perform operations.
const (
	RoleAdmin = Authorization(
		PermAccountView | PermAccountChangeEmail | PermAccountDisableMFA | PermAccountChangeLimits |
			PermAccountChangeName | PermAccountChangeKind | PermAccountChangeStatus | PermAccountCreateRestKey |
			PermAccountSetDataPlacement | PermAccountRemoveDataPlacement | PermAccountSetUserAgent |
			PermAccountSuspendTemporary | PermAccountReActivateTemporary | PermAccountSuspendPermanently |
			PermAccountReActivatePermanently | PermAccountDeleteNoData | PermAccountDeleteWithData | PermAccountMarkPendingDeletion |
			PermProjectView | PermProjectSetLimits | PermProjectSetDataPlacement | PermProjectUpdate |
			PermProjectRemoveDataPlacement | PermProjectSetUserAgent | PermProjectSendInvitation | PermProjectSetEntitlements |
			PermProjectDeleteNoData | PermProjectMarkPendingDeletion |
			PermBucketView | PermBucketSetDataPlacement | PermBucketRemoveDataPlacement |
			PermBucketSetUserAgent | PermViewChangeHistory,
	)
	RoleViewer          = Authorization(PermAccountView | PermProjectView | PermBucketView | PermViewChangeHistory)
	RoleCustomerSupport = Authorization(
		PermAccountView | PermAccountChangeEmail | PermAccountDisableMFA | PermAccountChangeLimits |
			PermAccountSetDataPlacement | PermAccountRemoveDataPlacement | PermAccountSetUserAgent |
			PermAccountSuspendTemporary | PermAccountReActivateTemporary | PermAccountDeleteNoData |
			PermProjectView | PermProjectSetLimits | PermProjectSetDataPlacement | PermProjectSetEntitlements |
			PermProjectRemoveDataPlacement | PermProjectSetUserAgent | PermProjectSendInvitation |
			PermBucketView | PermBucketSetDataPlacement | PermBucketRemoveDataPlacement |
			PermBucketSetUserAgent | PermViewChangeHistory,
	)
	RoleFinanceManager = Authorization(
		PermAccountView | PermAccountSuspendTemporary | PermAccountReActivateTemporary |
			PermAccountSuspendPermanently | PermAccountReActivatePermanently | PermAccountDeleteNoData |
			PermAccountDeleteWithData | PermProjectView | PermBucketView,
	)
)

// ErrAuthorizer is the error class that wraps all the errors returned by the authorization.
var ErrAuthorizer = errs.Class("authorizer")

// Permission represents a permissions to perform an operation.
type Permission uint64

// Authorization specifies the permissions that user role has and validates if it has certain
// permissions.
type Authorization uint64

// Has returns true if auth has all the passed permissions.
func (auth Authorization) Has(perms ...Permission) bool {
	for _, p := range perms {
		if uint64(auth)&uint64(p) == 0 {
			return false
		}
	}

	return true
}

// AuthInfo is the structure that holds information about the authenticated user.
type AuthInfo struct {
	Groups []string
	Email  string
}

// Authorizer checks if a group has certain permissions.
type Authorizer struct {
	log         *zap.Logger
	groupsRoles map[string]Authorization

	enabled     bool
	allowedHost string
}

// NewAuthorizer creates an Authorizer with the list of groups that are assigned to each different
// role. log is the parent logger where it will attach a prefix to identify messages coming from it.
//
// In the case that a group is assigned to more than one role, it will get the less permissive role.
func NewAuthorizer(
	log *zap.Logger,
	config Config,
) *Authorizer {
	groupsRoles := make(map[string]Authorization)

	// NOTE the order of iterating over all the groups matters because in the case that a group is in
	// more than one designed role, the group will get the role with less permissions that allow to
	// perform devastating operations.

	for _, g := range config.UserGroupsRoleAdmin {
		groupsRoles[g] = RoleAdmin
	}

	for _, g := range config.UserGroupsRoleFinanceManager {
		groupsRoles[g] = RoleFinanceManager
	}

	for _, g := range config.UserGroupsRoleCustomerSupport {
		groupsRoles[g] = RoleCustomerSupport
	}

	for _, g := range config.UserGroupsRoleViewer {
		groupsRoles[g] = RoleViewer
	}

	return &Authorizer{
		log:         log.Named("authorizer"),
		groupsRoles: groupsRoles,
		enabled:     !config.BypassAuth,
		allowedHost: config.AllowedOauthHost,
	}
}

// HasPermissions check if group has all perms.
func (auth *Authorizer) HasPermissions(group string, perms ...Permission) bool {
	if !auth.enabled {
		return true
	}
	groupAuth, ok := auth.groupsRoles[group]
	if !ok {
		return false
	}

	return groupAuth.Has(perms...)
}

// GetAuthInfo returns the information about the authenticated user from the request.
func (auth *Authorizer) GetAuthInfo(r *http.Request) *AuthInfo {
	if !auth.enabled {
		return &AuthInfo{Groups: []string{"bypass-auth"}, Email: "bypass@example.com"}
	}
	groups := r.Header.Get("X-Forwarded-Groups")
	if groups == "" {
		return nil
	}

	// Extract admin email from auth headers.
	email := r.Header.Get("X-Forwarded-Email")
	if email == "" {
		email = r.Header.Get("X-Auth-Request-Email")
	}

	return &AuthInfo{Groups: strings.Split(groups, ","), Email: email}
}

// IsRejected verifies that r is from a user who belongs to a group that has all perms and returns
// false, otherwise responds with http.StatusUnauthorized using
// storj.io/storj/private.api.ServeError and returns true.
//
// This method is convenient to inject it to the handlers generated by the API generator through a
// customized handler.
func (auth *Authorizer) IsRejected(w http.ResponseWriter, r *http.Request, perms ...Permission) bool {
	if !auth.enabled {
		return false
	}

	authInfo := auth.GetAuthInfo(r)
	if authInfo == nil || len(authInfo.Groups) == 0 {
		err := Error.Wrap(ErrAuthorizer.New("You do not belong to any group"))
		api.ServeError(auth.log, w, http.StatusUnauthorized, err)
		return true
	}
	if authInfo.Email == "" {
		err := Error.Wrap(ErrAuthorizer.New("missing user email"))
		api.ServeError(auth.log, w, http.StatusUnauthorized, err)
		return true
	}

	for _, g := range authInfo.Groups {
		if auth.HasPermissions(g, perms...) {
			return false
		}
	}

	err := Error.Wrap(ErrAuthorizer.New("Not enough permissions (your groups: %s)", strings.Join(authInfo.Groups, ",")))
	api.ServeError(auth.log, w, http.StatusUnauthorized, err)
	return true
}

// VerifyHost checks that the provided host is allowed to host the back office.
func (auth *Authorizer) VerifyHost(r *http.Request) error {
	if !auth.enabled {
		return nil
	}

	if r.Host != auth.allowedHost {
		return Error.New("forbidden host: %s", r.Host)
	}
	return nil
}
