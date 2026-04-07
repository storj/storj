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
	PermAccountChangeUpgradeTime
	PermAccountDisableMFA
	PermAccountChangeLimits
	PermAccountSetDataPlacement
	PermAccountRemoveDataPlacement
	PermAccountSetUserAgent
	PermAccountSuspend
	PermAccountReActivate
	PermAccountMarkPendingDeletion
	PermAccountDeleteNoData
	PermAccountCreateRestKey
	PermAccountDeleteWithData
	PermAccountCreateRegToken
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
	PermProjectMembersView
	PermViewChangeHistory
	PermNodesView
	PermAccountChangeLicenses
	PermAccountViewLicenses
	PermViewPrivateProjectID
	PermAccountUpdateTenantID
)

// These constants are the list of roles that users can have and the service uses to match
// permissions to perform operations.
const (
	RoleAdmin = Authorization(
		PermAccountView | PermAccountChangeEmail | PermAccountDisableMFA | PermAccountChangeLimits |
			PermAccountChangeName | PermAccountChangeKind | PermAccountChangeStatus | PermAccountCreateRestKey |
			PermAccountSetDataPlacement | PermAccountRemoveDataPlacement | PermAccountSetUserAgent |
			PermAccountSuspend | PermAccountReActivate | PermAccountDeleteNoData | PermAccountDeleteWithData |
			PermAccountMarkPendingDeletion | PermAccountCreateRegToken |
			PermProjectView | PermProjectSetLimits | PermProjectSetDataPlacement | PermProjectUpdate |
			PermProjectRemoveDataPlacement | PermProjectSetUserAgent | PermProjectSendInvitation | PermProjectSetEntitlements |
			PermProjectDeleteNoData | PermProjectMarkPendingDeletion |
			PermBucketView | PermBucketSetDataPlacement | PermBucketRemoveDataPlacement |
			PermBucketSetUserAgent | PermViewChangeHistory | PermAccountChangeUpgradeTime | PermNodesView | PermProjectMembersView |
			PermAccountChangeLicenses | PermAccountViewLicenses | PermViewPrivateProjectID | PermAccountUpdateTenantID,
	)
	RoleViewer = Authorization(
		PermAccountView | PermProjectView | PermBucketView | PermViewChangeHistory | PermProjectMembersView |
			PermAccountViewLicenses,
	)
	RoleCustomerSupport = Authorization(
		PermAccountView | PermAccountChangeEmail | PermAccountDisableMFA | PermAccountChangeLimits |
			PermAccountSetDataPlacement | PermAccountRemoveDataPlacement | PermAccountSetUserAgent |
			PermAccountSuspend | PermAccountReActivate | PermAccountDeleteNoData |
			PermProjectView | PermProjectSetLimits | PermProjectSetDataPlacement | PermProjectSetEntitlements |
			PermProjectRemoveDataPlacement | PermProjectSetUserAgent | PermProjectSendInvitation |
			PermBucketView | PermBucketSetDataPlacement | PermBucketRemoveDataPlacement |
			PermBucketSetUserAgent | PermViewChangeHistory | PermProjectMembersView | PermAccountChangeLicenses |
			PermAccountViewLicenses | PermAccountCreateRegToken | PermAccountChangeKind,
	)
	RoleFinanceManager = Authorization(
		PermAccountView | PermProjectView | PermBucketView | PermProjectMembersView |
			PermAccountViewLicenses,
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
	oidcMode    bool
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
		oidcMode:    config.OIDC.Enabled,
	}
}

// IsOIDCMode returns true if OIDC authentication is enabled.
func (auth *Authorizer) IsOIDCMode() bool {
	return auth.oidcMode
}

// IsAuthorized returns true if authInfo represents a valid.
// In OIDC mode, groups are not required.
func (auth *Authorizer) IsAuthorized(authInfo *AuthInfo) bool {
	if authInfo == nil {
		return false
	}
	if !auth.oidcMode && len(authInfo.Groups) == 0 {
		return false
	}
	return true
}

// HasPermissions check if group has all perms.
func (auth *Authorizer) HasPermissions(group string, perms ...Permission) bool {
	if !auth.enabled || auth.oidcMode {
		return true
	}
	groupAuth, ok := auth.groupsRoles[group]
	if !ok {
		return false
	}

	return groupAuth.Has(perms...)
}

// GroupsHavePerms checks if any of the groups has all permission.
func (auth *Authorizer) GroupsHavePerms(groups []string, perm ...Permission) bool {
	for _, g := range groups {
		if auth.HasPermissions(g, perm...) {
			return true
		}
	}
	return false
}

// GetAuthInfo returns the information about the authenticated user from the request.
func (auth *Authorizer) GetAuthInfo(r *http.Request) *AuthInfo {
	if !auth.enabled {
		return &AuthInfo{Groups: []string{"bypass-auth"}, Email: "bypass@example.com"}
	}

	// Extract admin email from auth headers.
	email := r.Header.Get("X-Forwarded-Email")
	if email == "" {
		email = r.Header.Get("X-Auth-Request-Email")
	}

	// In OIDC mode the middleware only sets X-Forwarded-Email; groups are not
	// used for authorization so we return a non-nil AuthInfo with just the email.
	if auth.oidcMode {
		return &AuthInfo{Email: email}
	}

	groups := r.Header.Get("X-Forwarded-Groups")
	if groups == "" {
		return nil
	}

	return &AuthInfo{Groups: strings.Split(groups, ","), Email: email}
}

// IsRejected verifies that r is from a user who belongs to a group that has all perms and returns
// false, otherwise responds with http.StatusUnauthorized using
// storj.io/storj/private.api.ServeError and returns true.
//
// In OIDC mode, group-based permission checks are skipped; any request with a
// valid authenticated email is allowed through.
//
// This method is convenient to inject it to the handlers generated by the API generator through a
// customized handler.
func (auth *Authorizer) IsRejected(w http.ResponseWriter, r *http.Request, perms ...Permission) bool {
	if !auth.enabled {
		return false
	}

	authInfo := auth.GetAuthInfo(r)

	// In OIDC mode every authenticated user has full access; only an
	// absent or empty email (unauthenticated request) is rejected.
	if auth.oidcMode {
		if authInfo == nil || authInfo.Email == "" {
			err := Error.Wrap(ErrAuthorizer.New("missing authentication"))
			api.ServeError(auth.log, w, http.StatusUnauthorized, err)
			return true
		}
		return false
	}

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
// The check is skipped when auth is disabled or when OIDC auth is enabled, since
// OIDC mode uses cookie based session verification.
func (auth *Authorizer) VerifyHost(r *http.Request) error {
	if !auth.enabled || auth.oidcMode {
		return nil
	}

	if r.Host != auth.allowedHost {
		return Error.New("forbidden host: %s", r.Host)
	}
	return nil
}
