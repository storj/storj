// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"bytes"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	admin "storj.io/storj/satellite/admin"
)

func TestAuthorization(t *testing.T) {
	permissions := "10001001010111011011"
	a, err := strconv.ParseUint(permissions, 2, 64)
	require.NoError(t, err)
	auth := admin.Authorization(a)

	var (
		permsOn  []admin.Permission
		permsOff []admin.Permission
	)

	for i, b := range permissions {
		perm := admin.Permission(1 << (len(permissions) - 1 - i))

		if b == '1' {
			permsOn = append(permsOn, perm)
			require.True(t, auth.Has(perm), "has permission")
		} else {
			permsOff = append(permsOff, perm)
			require.False(t, auth.Has(perm), "doesn't have permission")
		}
	}

	rand.Shuffle(len(permsOn), func(i, j int) {
		permsOn[i], permsOn[j] = permsOn[j], permsOn[i]
	})

	rand.Shuffle(len(permsOff), func(i, j int) {
		permsOff[i], permsOff[j] = permsOff[j], permsOff[i]
	})

	require.True(t, auth.Has(permsOn...), "full list of permissions that has")
	require.False(t, auth.Has(permsOff...), "full list of permissions that doesn't have")

	permOnIdx := rand.Intn(len(permsOn))
	permOffIdx := rand.Intn(len(permsOff))

	require.False(t, auth.Has(permsOn[permOnIdx], permsOff[permOffIdx]))
}

func TestAuthorizer(t *testing.T) {
	ctx := testcontext.New(t)
	log := zaptest.NewLogger(t)
	gropusAdmin := []string{"root", "super"}
	groupsViewer := []string{"everyone", "everyone-else"}
	groupsSupport := []string{"customers-success", "customers-troubleshooter"}
	groupsFinance := []string{"accountant"}

	cases := []struct {
		name        string
		group       string
		permissions []admin.Permission
		hasAccess   bool
	}{
		{
			name:        "root deletes account with data",
			group:       "root",
			permissions: []admin.Permission{admin.PermAccountDeleteWithData},
			hasAccess:   true,
		},
		{
			name:        "super re-activates account permanently",
			group:       "super",
			permissions: []admin.Permission{admin.PermAccountDeleteWithData},
			hasAccess:   true,
		},
		{
			name:        "everyone view account data",
			group:       "everyone",
			permissions: []admin.Permission{admin.PermAccountView},
			hasAccess:   true,
		},
		{
			name:        "everyone removes project data placement",
			group:       "everyone",
			permissions: []admin.Permission{admin.PermProjectRemoveDataPlacement},
			hasAccess:   false,
		},
		{
			name:        "everyone-else views bucket data",
			group:       "everyone-else",
			permissions: []admin.Permission{admin.PermBucketView},
			hasAccess:   true,
		},
		{
			name:        "everyone-else sets project user agent",
			group:       "everyone-else",
			permissions: []admin.Permission{admin.PermProjectSetUserAgent},
			hasAccess:   false,
		},
		{
			name:        "customers-success suspends account",
			group:       "customers-success",
			permissions: []admin.Permission{admin.PermAccountSuspend},
			hasAccess:   true,
		},
		{
			name:        "customers-troubleshooter suspends account and sets project limits",
			group:       "customers-troubleshooter",
			permissions: []admin.Permission{admin.PermAccountSuspend, admin.PermProjectSetLimits},
			hasAccess:   true,
		},
		{
			name:        "customers-troubleshooter suspends account and deletes account with data",
			group:       "customers-troubleshooter",
			permissions: []admin.Permission{admin.PermAccountSuspend, admin.PermAccountDeleteWithData},
			hasAccess:   false,
		},
		{
			name:        "customers-success creates reg token",
			group:       "customers-success",
			permissions: []admin.Permission{admin.PermAccountCreateRegToken},
			hasAccess:   true,
		},
		{
			name:        "customers-troubleshooter creates reg token",
			group:       "customers-troubleshooter",
			permissions: []admin.Permission{admin.PermAccountCreateRegToken},
			hasAccess:   true,
		},
		{
			name:        "customers-troubleshooter changes user kind",
			group:       "customers-troubleshooter",
			permissions: []admin.Permission{admin.PermAccountChangeKind},
			hasAccess:   true,
		},
		{
			name:        "accountant suspends account",
			group:       "accountant",
			permissions: []admin.Permission{admin.PermAccountSuspend},
			hasAccess:   false,
		},
		{
			name:        "accountant sets bucket user agent",
			group:       "accountant",
			permissions: []admin.Permission{admin.PermBucketSetUserAgent},
			hasAccess:   false,
		},
		{
			name:        "accountant creates reg token",
			group:       "accountant",
			permissions: []admin.Permission{admin.PermAccountCreateRegToken},
			hasAccess:   false,
		},
	}

	config := admin.Config{
		UserGroupsRoleAdmin:           gropusAdmin,
		UserGroupsRoleViewer:          groupsViewer,
		UserGroupsRoleCustomerSupport: groupsSupport,
		UserGroupsRoleFinanceManager:  groupsFinance,
	}
	auth := admin.NewAuthorizer(log, config)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Run("HasPermissions", func(t *testing.T) {
				authInfo := &admin.AuthInfo{Groups: []string{c.group}}
				require.Equal(t, c.hasAccess, auth.HasPermissions(authInfo, c.permissions...))
			})

			t.Run("isRejected", func(t *testing.T) {
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.test", nil)
				require.NoError(t, err)
				req.Header.Add("X-Forwarded-Groups", c.group)
				req.Header.Add("X-Forwarded-Email", "test@example.com")
				w := httptest.NewRecorder()
				wbuff := &bytes.Buffer{}
				w.Body = wbuff

				handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if auth.IsRejected(w, r, c.permissions...) {
						return
					}
					w.WriteHeader(http.StatusOK)
				})
				handler.ServeHTTP(w, req)

				if c.hasAccess {
					assert.Equal(t, http.StatusOK, w.Code, "HTTP Status Code")
				} else {
					assert.Equal(t, http.StatusUnauthorized, w.Code, "HTTP Status Code")
					assert.Contains(t, wbuff.String(), "Not enough permissions")
				}
			})
		})
	}

	t.Run("IsRejected request with multiple groups one has the permissions", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.test", nil)
		require.NoError(t, err)
		req.Header.Add("X-Forwarded-Groups", "everyone-else,super")
		req.Header.Add("X-Forwarded-Email", "test@example.com")
		w := httptest.NewRecorder()
		wbuff := &bytes.Buffer{}
		w.Body = wbuff

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if auth.IsRejected(w, r, admin.PermAccountDeleteWithData) {
				return
			}
			w.WriteHeader(http.StatusOK)
		})
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "HTTP Status Code")
	})

	t.Run("IsRejected request with multiple groups none has all the permissions", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.test", nil)
		require.NoError(t, err)
		req.Header.Add("X-Forwarded-Groups", "customers-troubleshooter,everyone-else")
		req.Header.Add("X-Forwarded-Email", "test@example.com")
		w := httptest.NewRecorder()
		wbuff := &bytes.Buffer{}
		w.Body = wbuff

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if auth.IsRejected(w, r, admin.PermAccountDeleteWithData) {
				return
			}
			w.WriteHeader(http.StatusOK)
		})
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code, "HTTP Status Code")
		assert.Contains(t, wbuff.String(), "Not enough permissions")
	})

	t.Run("IsRejected request with email in emailsRoles that has sufficient permissions", func(t *testing.T) {
		emailConfig := admin.Config{
			UserEmailsRoleAdmin: []string{"test@example.com"},
		}
		emailAuth := admin.NewAuthorizer(log, emailConfig)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.test", nil)
		require.NoError(t, err)
		req.Header.Add("X-Forwarded-Email", "test@example.com")
		w := httptest.NewRecorder()

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if emailAuth.IsRejected(w, r, admin.PermAccountDeleteWithData) {
				return
			}
			w.WriteHeader(http.StatusOK)
		})
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "email-only admin should be allowed")
	})

	t.Run("IsRejected request with email in emailsRoles that lacks required permissions", func(t *testing.T) {
		emailConfig := admin.Config{
			UserEmailsRoleViewer: []string{"viewer@example.com"},
		}
		emailAuth := admin.NewAuthorizer(log, emailConfig)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.test", nil)
		require.NoError(t, err)
		req.Header.Add("X-Forwarded-Email", "viewer@example.com")
		w := httptest.NewRecorder()
		wbuff := &bytes.Buffer{}
		w.Body = wbuff

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if emailAuth.IsRejected(w, r, admin.PermAccountDeleteWithData) {
				return
			}
			w.WriteHeader(http.StatusOK)
		})
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "viewer email should be denied write perm")
	})

	t.Run("IsRejected request with a unauthorized group", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.test", nil)
		require.NoError(t, err)
		req.Header.Add("X-Forwarded-Groups", "engineering")
		req.Header.Add("X-Forwarded-Email", "test@example.com")
		w := httptest.NewRecorder()
		wbuff := &bytes.Buffer{}
		w.Body = wbuff

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if auth.IsRejected(w, r, admin.PermAccountDeleteWithData) {
				return
			}
			w.WriteHeader(http.StatusOK)
		})
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code, "HTTP Status Code")
		assert.Contains(t, wbuff.String(), "Not enough permissions")
	})

	t.Run("IsRejected request with no headers (unauthenticated)", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.test", nil)
		require.NoError(t, err)
		w := httptest.NewRecorder()
		wbuff := &bytes.Buffer{}
		w.Body = wbuff

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if auth.IsRejected(w, r, admin.PermAccountDeleteWithData) {
				return
			}
			w.WriteHeader(http.StatusOK)
		})
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code, "HTTP Status Code")
		assert.Contains(t, wbuff.String(), "authentication required")
	})

	t.Run("IsRejected request with email but no groups", func(t *testing.T) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://example.test", nil)
		require.NoError(t, err)
		req.Header.Add("X-Forwarded-Email", "unknown@example.com")
		w := httptest.NewRecorder()
		wbuff := &bytes.Buffer{}
		w.Body = wbuff

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if auth.IsRejected(w, r, admin.PermAccountDeleteWithData) {
				return
			}
			w.WriteHeader(http.StatusOK)
		})
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code, "HTTP Status Code")
		assert.Contains(t, wbuff.String(), "Not enough permissions")
	})
}

func TestAuthorizerEmailRoles(t *testing.T) {
	log := zaptest.NewLogger(t)

	config := admin.Config{
		UserEmailsRoleAdmin:           []string{"root@example.com"},
		UserEmailsRoleViewer:          []string{"viewer@example.com"},
		UserEmailsRoleCustomerSupport: []string{"support@example.com"},
		UserEmailsRoleFinanceManager:  []string{"finance@example.com"},
	}
	auth := admin.NewAuthorizer(log, config)

	authInfoFor := func(email string) *admin.AuthInfo {
		return &admin.AuthInfo{Email: email}
	}

	t.Run("admin email has admin permissions", func(t *testing.T) {
		require.True(t, auth.HasPermissions(authInfoFor("root@example.com"), admin.PermAccountDeleteWithData))
		require.True(t, auth.HasPermissions(authInfoFor("root@example.com"), admin.PermAccountView, admin.PermProjectSetLimits))
	})

	t.Run("viewer email has read-only permissions", func(t *testing.T) {
		require.True(t, auth.HasPermissions(authInfoFor("viewer@example.com"), admin.PermAccountView))
		require.False(t, auth.HasPermissions(authInfoFor("viewer@example.com"), admin.PermAccountDeleteWithData))
		require.False(t, auth.HasPermissions(authInfoFor("viewer@example.com"), admin.PermProjectSetLimits))
	})

	t.Run("support email has customer support permissions", func(t *testing.T) {
		require.True(t, auth.HasPermissions(authInfoFor("support@example.com"), admin.PermAccountSuspend))
		require.True(t, auth.HasPermissions(authInfoFor("support@example.com"), admin.PermProjectSetLimits))
		require.False(t, auth.HasPermissions(authInfoFor("support@example.com"), admin.PermAccountDeleteWithData))
	})

	t.Run("finance email has finance manager permissions", func(t *testing.T) {
		require.True(t, auth.HasPermissions(authInfoFor("finance@example.com"), admin.PermAccountView))
		require.False(t, auth.HasPermissions(authInfoFor("finance@example.com"), admin.PermAccountSuspend))
	})

	t.Run("unconfigured email has no permissions", func(t *testing.T) {
		require.False(t, auth.HasPermissions(authInfoFor("stranger@example.com"), admin.PermAccountView))
	})

	t.Run("email in multiple roles gets least permissive (viewer overwrites admin)", func(t *testing.T) {
		// viewer is processed last so it overwrites admin for the same email.
		overlapConfig := admin.Config{
			UserEmailsRoleAdmin:  []string{"dual@example.com"},
			UserEmailsRoleViewer: []string{"dual@example.com"},
		}
		overlapAuth := admin.NewAuthorizer(log, overlapConfig)
		require.False(t, overlapAuth.HasPermissions(authInfoFor("dual@example.com"), admin.PermAccountDeleteWithData),
			"viewer role should have overridden admin for the duplicate email")
		require.True(t, overlapAuth.HasPermissions(authInfoFor("dual@example.com"), admin.PermAccountView),
			"viewer permissions should still apply")
	})
}
