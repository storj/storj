// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"bytes"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/testcontext"
	admin "storj.io/storj/satellite/admin/back-office"
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
			name:        "customers-success suspends account temporary",
			group:       "customers-success",
			permissions: []admin.Permission{admin.PermAccountSuspendTemporary},
			hasAccess:   true,
		},
		{
			name:        "customers-success  suspends account permanently",
			group:       "customers-success",
			permissions: []admin.Permission{admin.PermAccountSuspendPermanently},
			hasAccess:   false,
		},
		{
			name:        "customers-troubleshooter suspends account temporary and sets project limits",
			group:       "customers-troubleshooter",
			permissions: []admin.Permission{admin.PermAccountSuspendTemporary, admin.PermProjectSetLimits},
			hasAccess:   true,
		},
		{
			name:        "customers-troubleshooter suspends account temporary and deletes account with data",
			group:       "customers-troubleshooter",
			permissions: []admin.Permission{admin.PermAccountSuspendTemporary, admin.PermAccountDeleteWithData},
			hasAccess:   false,
		},
		{
			name:        "accountant suspends account permanently",
			group:       "accountant",
			permissions: []admin.Permission{admin.PermAccountSuspendPermanently},
			hasAccess:   true,
		},
		{
			name:        "accountant sets bucket user agent",
			group:       "accountant",
			permissions: []admin.Permission{admin.PermBucketSetUserAgent},
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
				require.Equal(t, c.hasAccess, auth.HasPermissions(c.group, c.permissions...))
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
					assert.Contains(t, wbuff.String(), fmt.Sprintf(`Not enough permissions (your groups: %s)`, c.group))
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
		assert.Contains(t, wbuff.String(), `Not enough permissions (your groups: customers-troubleshooter,everyone-else)`)
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
		assert.Contains(t, wbuff.String(), `Not enough permissions (your groups: engineering)`)
	})

	t.Run("IsRejected request with no groups", func(t *testing.T) {
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
		assert.Contains(t, wbuff.String(), "You do not belong to any group")
	})
}
