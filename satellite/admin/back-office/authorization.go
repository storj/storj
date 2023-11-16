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
	PermAccountDisableMFA
	PermAccountChangeLimits
	PermAccountSetDataPlacement
	PermAccountRemoveDataPlacement
	PermAccountSetUserAgent
	PermAccountSuspendTemporary
	PermAccountReActivateTemporary
	PermAccountSuspendPermanently
	PermAccountReActivatePermanently
	PermAccountDeleteNoData
	PermAccountDeleteWithData
	PermProjectView
	PermProjectSetLimits
	PermProjectSetDataPlacement
	PermProjectRemoveDataPlacement
	PermProjectSetUserAgent
	PermProjectSendInvitation
	PermBucketView
	PermBucketSetDataPlacement
	PermBucketRemoveDataPlacement
	PermBucketSetUserAgent
)

// These constants are the list of roles that users can have and the service uses to match
// permissions to perform operations.
const (
	RoleAdmin = Authorization(
		PermAccountView | PermAccountChangeEmail | PermAccountDisableMFA | PermAccountChangeLimits |
			PermAccountSetDataPlacement | PermAccountRemoveDataPlacement | PermAccountSetUserAgent |
			PermAccountSuspendTemporary | PermAccountReActivateTemporary | PermAccountSuspendPermanently |
			PermAccountReActivatePermanently | PermAccountDeleteNoData | PermAccountDeleteWithData |
			PermProjectView | PermProjectSetLimits | PermProjectSetDataPlacement |
			PermProjectRemoveDataPlacement | PermProjectSetUserAgent | PermProjectSendInvitation |
			PermBucketView | PermBucketSetDataPlacement | PermBucketRemoveDataPlacement |
			PermBucketSetUserAgent,
	)
	RoleViewer          = Authorization(PermAccountView | PermProjectView | PermBucketView)
	RoleCustomerSupport = Authorization(
		PermAccountView | PermAccountChangeEmail | PermAccountDisableMFA | PermAccountChangeLimits |
			PermAccountSetDataPlacement | PermAccountRemoveDataPlacement | PermAccountSetUserAgent |
			PermAccountSuspendTemporary | PermAccountReActivateTemporary | PermAccountDeleteNoData |
			PermProjectView | PermProjectSetLimits | PermProjectSetDataPlacement |
			PermProjectRemoveDataPlacement | PermProjectSetUserAgent | PermProjectSendInvitation |
			PermBucketView | PermBucketSetDataPlacement | PermBucketRemoveDataPlacement |
			PermBucketSetUserAgent,
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

// Authorizer checks if a group has certain permissions.
type Authorizer struct {
	log         *zap.Logger
	groupsRoles map[string]Authorization
}

// NewAuthorizer creates an Authorizer with the list of groups that are assigned to each different
// role. log is the parent logger where it will attach a prefix to identify messages coming from it.
//
// In the case that a group is assigned to more than one role, it will get the less permissive role.
func NewAuthorizer(
	log *zap.Logger,
	adminGroups, viewerGroups, customerSupportGroups, financeManagerGroups []string,
) *Authorizer {
	groupsRoles := make(map[string]Authorization)

	// NOTE the order of iterating over all the groups matters because in the case that a group is in
	// more than one designed role, the group will get the role with less permissions that allow to
	// perform devastating operations.

	for _, g := range adminGroups {
		groupsRoles[g] = RoleAdmin
	}

	for _, g := range financeManagerGroups {
		groupsRoles[g] = RoleFinanceManager
	}

	for _, g := range customerSupportGroups {
		groupsRoles[g] = RoleCustomerSupport
	}

	for _, g := range viewerGroups {
		groupsRoles[g] = RoleViewer
	}

	return &Authorizer{
		log:         log.Named("authorizer"),
		groupsRoles: groupsRoles,
	}
}

// HasPermissions check if group has all perms.
func (auth *Authorizer) HasPermissions(group string, perms ...Permission) bool {
	groupAuth, ok := auth.groupsRoles[group]
	if !ok {
		return false
	}

	return groupAuth.Has(perms...)
}

// Middleware returns an HTTP handler which verifies if the request is performed by a user with a
// role that allows all the passed permissions.
func (auth *Authorizer) Middleware(next http.Handler, perms ...Permission) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		groupsh := r.Header.Get("X-Forwarded-Groups")
		if groupsh == "" {
			err := Error.Wrap(ErrAuthorizer.New("You do not belong to any group"))
			api.ServeError(auth.log, w, http.StatusUnauthorized, err)
			return
		}

		groups := strings.Split(groupsh, ",")
		for _, g := range groups {
			if auth.HasPermissions(g, perms...) {
				next.ServeHTTP(w, r)
				return
			}
		}

		err := Error.Wrap(ErrAuthorizer.New("Not enough permissions (your groups: %s)", groupsh))
		api.ServeError(auth.log, w, http.StatusUnauthorized, err)
	})
}
