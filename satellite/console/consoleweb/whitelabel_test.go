// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/tenancy"
)

// TestUserIsolationAcrossTenants verifies that the same email can create
// separate accounts on different tenants and that users are properly isolated.
func TestUserIsolationAcrossTenants(t *testing.T) {
	const (
		tenant1ID   = "tenant1"
		tenant2ID   = "tenant2"
		sharedEmail = "user@example.com"
		password    = "password123"
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		usersDB := sat.DB.Console().Users()

		var (
			tenant1IDStr = tenant1ID
			tenant2IDStr = tenant2ID
		)

		t.Run("Create user on tenant1", func(t *testing.T) {
			ctx1 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenant1ID})

			user1, err := service.CreateUser(ctx1, console.CreateUser{
				FullName: "User On Tenant 1",
				Email:    sharedEmail,
				Password: password,
			}, nil)
			require.NoError(t, err)
			require.NotNil(t, user1)
			require.Equal(t, sharedEmail, user1.Email)
			require.NotNil(t, user1.TenantID)
			require.Equal(t, tenant1ID, *user1.TenantID)
		})

		t.Run("Create user with same email on tenant2", func(t *testing.T) {
			ctx2 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenant2ID})

			user2, err := service.CreateUser(ctx2, console.CreateUser{
				FullName: "User On Tenant 2",
				Email:    sharedEmail,
				Password: password,
			}, nil)
			require.NoError(t, err)
			require.NotNil(t, user2)
			require.Equal(t, sharedEmail, user2.Email)
			require.NotNil(t, user2.TenantID)
			require.Equal(t, tenant2ID, *user2.TenantID)
		})

		t.Run("Verify separate users in database", func(t *testing.T) {
			tenant1IDPtr := &tenant1IDStr
			user1, unverified1, err := usersDB.GetByEmailAndTenantWithUnverified(ctx, sharedEmail, tenant1IDPtr)
			require.NoError(t, err)
			if user1 == nil {
				require.Len(t, unverified1, 1)
				user1 = &unverified1[0]
			}
			require.NotNil(t, user1)
			require.Equal(t, tenant1ID, *user1.TenantID)
			require.Equal(t, "User On Tenant 1", user1.FullName)

			tenant2IDPtr := &tenant2IDStr
			user2, unverified2, err := usersDB.GetByEmailAndTenantWithUnverified(ctx, sharedEmail, tenant2IDPtr)
			require.NoError(t, err)
			if user2 == nil {
				require.Len(t, unverified2, 1)
				user2 = &unverified2[0]
			}
			require.NotNil(t, user2)
			require.Equal(t, tenant2ID, *user2.TenantID)
			require.Equal(t, "User On Tenant 2", user2.FullName)

			require.NotEqual(t, user1.ID, user2.ID)
		})

		t.Run("Create user with same email on default tenant", func(t *testing.T) {
			userDefault, err := service.CreateUser(ctx, console.CreateUser{
				FullName: "User On Default Tenant",
				Email:    sharedEmail,
				Password: password,
			}, nil)
			require.NoError(t, err)
			require.NotNil(t, userDefault)
			require.Equal(t, sharedEmail, userDefault.Email)
			require.Nil(t, userDefault.TenantID)
		})

		t.Run("Duplicate email within same tenant fails", func(t *testing.T) {
			ctx1 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenant1ID})

			tenant1IDPtr := &tenant1IDStr
			existingUser, unverified, err := usersDB.GetByEmailAndTenantWithUnverified(ctx, sharedEmail, tenant1IDPtr)
			require.NoError(t, err)
			if existingUser == nil {
				require.Len(t, unverified, 1)
				existingUser = &unverified[0]
			}
			existingUser.Status = console.Active
			err = usersDB.Update(ctx1, existingUser.ID, console.UpdateUserRequest{
				Status: &existingUser.Status,
			})
			require.NoError(t, err)

			_, err = service.CreateUser(ctx1, console.CreateUser{
				FullName: "Duplicate User",
				Email:    sharedEmail,
				Password: password,
			}, nil)
			require.Error(t, err)
			require.True(t, console.ErrEmailUsed.Has(err))
		})
	})
}

// TestAuthenticationTenantIsolation verifies that login is tenant-aware and users can only authenticate within their tenant context.
func TestAuthenticationTenantIsolation(t *testing.T) {
	const (
		tenant1ID = "tenant1"
		tenant2ID = "tenant2"
		email     = "user@example.com"
		password  = "password123"
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		usersDB := sat.DB.Console().Users()

		ctx1 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenant1ID})

		user1, err := service.CreateUser(ctx1, console.CreateUser{
			FullName: "User One",
			Email:    email,
			Password: password,
		}, nil)
		require.NoError(t, err)

		user1.Status = console.Active
		err = usersDB.Update(ctx1, user1.ID, console.UpdateUserRequest{
			Status: &user1.Status,
		})
		require.NoError(t, err)

		ctx2 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenant2ID})

		user2, err := service.CreateUser(ctx2, console.CreateUser{
			FullName: "User Two",
			Email:    email,
			Password: password,
		}, nil)
		require.NoError(t, err)

		user2.Status = console.Active
		err = usersDB.Update(ctx2, user2.ID, console.UpdateUserRequest{
			Status: &user2.Status,
		})
		require.NoError(t, err)

		t.Run("Login on tenant1", func(t *testing.T) {
			tokenInfo, err := service.Token(ctx1, console.AuthUser{
				Email:    email,
				Password: password,
			})
			require.NoError(t, err)
			require.NotNil(t, tokenInfo)
		})

		t.Run("Login on tenant2", func(t *testing.T) {
			tokenInfo, err := service.Token(ctx2, console.AuthUser{
				Email:    email,
				Password: password,
			})
			require.NoError(t, err)
			require.NotNil(t, tokenInfo)
		})

		t.Run("Login without tenant context fails", func(t *testing.T) {
			_, err := service.Token(ctx, console.AuthUser{
				Email:    email,
				Password: password,
			})
			require.Error(t, err)
			require.True(t, console.ErrLoginCredentials.Has(err))
		})
	})
}

// TestCrossTenantDataAccessPrevention verifies that users cannot access data (projects, buckets, etc.) from other tenants.
func TestCrossTenantDataAccessPrevention(t *testing.T) {
	const (
		tenant1ID  = "tenant1"
		tenant2ID  = "tenant2"
		user1Email = "user1@example.com"
		user2Email = "user2@example.com"
		password   = "password123"
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()

		ctx1 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenant1ID})

		user1, err := service.CreateUser(ctx1, console.CreateUser{
			FullName: "User One",
			Email:    user1Email,
			Password: password,
		}, nil)
		require.NoError(t, err)

		user1.Status = console.Active
		err = usersDB.Update(ctx1, user1.ID, console.UpdateUserRequest{
			Status: &user1.Status,
		})
		require.NoError(t, err)

		ctx2 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenant2ID})

		user2, err := service.CreateUser(ctx2, console.CreateUser{
			FullName: "User Two",
			Email:    user2Email,
			Password: password,
		}, nil)
		require.NoError(t, err)

		user2.Status = console.Active
		err = usersDB.Update(ctx2, user2.ID, console.UpdateUserRequest{
			Status: &user2.Status,
		})
		require.NoError(t, err)

		ctx1WithUser := console.WithUser(ctx1, user1)
		project1, err := service.CreateProject(ctx1WithUser, console.UpsertProjectInfo{
			Name:        "Tenant 1 Project",
			Description: "Project on tenant 1",
		})
		require.NoError(t, err)
		require.NotNil(t, project1)

		ctx2WithUser := console.WithUser(ctx2, user2)
		project2, err := service.CreateProject(ctx2WithUser, console.UpsertProjectInfo{
			Name:        "Tenant 2 Project",
			Description: "Project on tenant 2",
		})
		require.NoError(t, err)
		require.NotNil(t, project2)

		t.Run("User can access own project on same tenant", func(t *testing.T) {
			projects, err := service.GetUsersProjects(ctx1WithUser)
			require.NoError(t, err)
			require.Len(t, projects, 1)
			require.Equal(t, project1.ID, projects[0].ID)
		})

		t.Run("User on different tenant can access own project", func(t *testing.T) {
			projects, err := service.GetUsersProjects(ctx2WithUser)
			require.NoError(t, err)
			require.Len(t, projects, 1)
			require.Equal(t, project2.ID, projects[0].ID)
		})

		t.Run("Database queries respect tenant isolation", func(t *testing.T) {
			user1Projects, err := projectsDB.GetByUserID(ctx1, user1.ID)
			require.NoError(t, err)
			require.Len(t, user1Projects, 1)
			require.Equal(t, project1.ID, user1Projects[0].ID)

			user2Projects, err := projectsDB.GetByUserID(ctx2, user2.ID)
			require.NoError(t, err)
			require.Len(t, user2Projects, 1)
			require.Equal(t, project2.ID, user2Projects[0].ID)

			require.NotEqual(t, project1.ID, project2.ID)
		})

		t.Run("User cannot access project from different tenant", func(t *testing.T) {
			_, err := service.GetProject(ctx1WithUser, project2.ID)
			require.Error(t, err)
			require.True(t, console.ErrNoMembership.Has(err))

			_, err = service.GetProject(ctx2WithUser, project1.ID)
			require.Error(t, err)
			require.True(t, console.ErrNoMembership.Has(err))
		})
	})
}

// TestBrandingAPIHostHeaders verifies that the branding API handles various
// Host header formats correctly, returning the default branding in each case.
func TestBrandingAPIHostHeaders(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		addr := sat.API.Console.Listener.Addr().String()
		client := http.DefaultClient

		t.Run("Default branding for unknown host", func(t *testing.T) {
			url := "http://" + addr + "/api/v0/config/branding"
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
			require.NoError(t, err)

			req.Host = "unknown.example.com"

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var branding map[string]any
			require.NoError(t, json.Unmarshal(bodyBytes, &branding))

			require.Equal(t, "Storj", branding["name"])
		})

		t.Run("Case insensitive hostname", func(t *testing.T) {
			url := "http://" + addr + "/api/v0/config/branding"
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
			require.NoError(t, err)

			req.Host = strings.ToUpper("example.com")

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var branding map[string]any
			require.NoError(t, json.Unmarshal(bodyBytes, &branding))

			require.Equal(t, "Storj", branding["name"])
		})

		t.Run("Hostname with port number", func(t *testing.T) {
			url := "http://" + addr + "/api/v0/config/branding"
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
			require.NoError(t, err)

			req.Host = "example.com:8080"

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var branding map[string]any
			require.NoError(t, json.Unmarshal(bodyBytes, &branding))

			require.Equal(t, "Storj", branding["name"])
		})
	})
}

// TestDefaultExternalAddressInInviteLinks verifies that invite links use the
// configured global external address when no SingleWhiteLabel is set.
func TestDefaultExternalAddressInInviteLinks(t *testing.T) {
	const (
		ownerEmail   = "owner@example.com"
		inviteeEmail = "invitee@example.com"
		password     = "password123"
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		usersDB := sat.DB.Console().Users()

		owner, err := service.CreateUser(ctx, console.CreateUser{
			FullName: "Default Owner",
			Email:    ownerEmail,
			Password: password,
		}, nil)
		require.NoError(t, err)

		owner.Status = console.Active
		err = usersDB.Update(ctx, owner.ID, console.UpdateUserRequest{
			Status: &owner.Status,
		})
		require.NoError(t, err)
		err = usersDB.UpdatePaidTier(ctx, owner.ID, true, 0, 0, 0, 10, nil)
		require.NoError(t, err)

		owner, err = usersDB.Get(ctx, owner.ID)
		require.NoError(t, err)

		ctxWithOwner := console.WithUser(ctx, owner)
		project, err := service.CreateProject(ctxWithOwner, console.UpsertProjectInfo{
			Name: "Default Project",
		})
		require.NoError(t, err)

		_, err = service.InviteNewProjectMember(ctxWithOwner, project.ID, inviteeEmail)
		require.NoError(t, err)

		link, err := service.GetInviteLink(ctxWithOwner, project.PublicID, inviteeEmail)
		require.NoError(t, err)

		require.NotEmpty(t, link)
		require.Contains(t, link, "/invited?invite=")
		require.True(t, strings.HasPrefix(link, sat.Config.Console.ExternalAddress),
			"invite link should use the global external address, got: %s", link)
	})
}

// TestSingleWhiteLabelTenantContext verifies that SingleWhiteLabel mode
// properly sets the tenant ID in context for all requests.
func TestSingleWhiteLabelTenantContext(t *testing.T) {
	const (
		singleTenantID     = "single-brand"
		singleTenantName   = "Single Brand Co"
		singleExternalAddr = "https://console.example.test/"
		sharedEmail        = "user@example.com"
		password           = "password123"
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				// Configure SingleWhiteLabel mode - no multi-tenant config needed.
				config.Console.SingleWhiteLabel = console.SingleWhiteLabelConfig{
					TenantID:        singleTenantID,
					Name:            singleTenantName,
					ExternalAddress: singleExternalAddr,
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		usersDB := sat.DB.Console().Users()

		// Create a variable for the tenant ID so we can take its address.
		tenantIDStr := singleTenantID

		t.Run("User created with SingleWhiteLabel tenant ID", func(t *testing.T) {
			// In SingleWhiteLabel mode, all requests should use the configured tenant ID.
			// Simulate a request coming through the middleware by setting context.
			tenantCtx := tenancy.WithContext(ctx, &tenancy.Context{TenantID: singleTenantID})

			user, err := service.CreateUser(tenantCtx, console.CreateUser{
				FullName: "Single Brand User",
				Email:    sharedEmail,
				Password: password,
			}, nil)
			require.NoError(t, err)
			require.NotNil(t, user)
			require.Equal(t, sharedEmail, user.Email)
			require.NotNil(t, user.TenantID)
			require.Equal(t, singleTenantID, *user.TenantID)
		})

		t.Run("User lookup uses SingleWhiteLabel tenant ID", func(t *testing.T) {
			tenantCtx := tenancy.WithContext(ctx, &tenancy.Context{TenantID: singleTenantID})

			// Look up user by email - should find the user created above.
			user, unverified, err := usersDB.GetByEmailAndTenantWithUnverified(tenantCtx, sharedEmail, &tenantIDStr)
			require.NoError(t, err)
			// User was just created and not activated, so they're in the unverified list.
			if user == nil {
				require.Len(t, unverified, 1)
				user = &unverified[0]
			}
			require.NotNil(t, user)
			require.Equal(t, sharedEmail, user.Email)
		})

		t.Run("Different tenant context cannot find SingleWhiteLabel user", func(t *testing.T) {
			differentTenantID := "other-tenant"
			otherCtx := tenancy.WithContext(ctx, &tenancy.Context{TenantID: differentTenantID})

			// Look up user with different tenant ID - should NOT find the user.
			user, unverified, err := usersDB.GetByEmailAndTenantWithUnverified(otherCtx, sharedEmail, &differentTenantID)
			require.NoError(t, err)
			require.Nil(t, user)
			require.Empty(t, unverified)
		})
	})
}

// TestSingleWhiteLabelExternalAddress verifies that SingleWhiteLabel mode
// uses the configured external address in invite links.
func TestSingleWhiteLabelExternalAddress(t *testing.T) {
	const (
		singleTenantID     = "single-brand"
		singleTenantName   = "Single Brand Co"
		singleExternalAddr = "https://console.example.test"
		ownerEmail         = "owner@example.com"
		inviteeEmail       = "invitee@example.com"
		password           = "password123"
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.SingleWhiteLabel = console.SingleWhiteLabelConfig{
					TenantID:        singleTenantID,
					Name:            singleTenantName,
					ExternalAddress: singleExternalAddr,
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		usersDB := sat.DB.Console().Users()

		// Create and activate owner.
		tenantCtx := tenancy.WithContext(ctx, &tenancy.Context{TenantID: singleTenantID})

		owner, err := service.CreateUser(tenantCtx, console.CreateUser{
			FullName: "Project Owner",
			Email:    ownerEmail,
			Password: password,
		}, nil)
		require.NoError(t, err)

		owner.Status = console.Active
		err = usersDB.Update(tenantCtx, owner.ID, console.UpdateUserRequest{
			Status: &owner.Status,
		})
		require.NoError(t, err)
		err = usersDB.UpdatePaidTier(tenantCtx, owner.ID, true, 0, 0, 0, 10, nil)
		require.NoError(t, err)

		// Refresh user to get updated limits.
		owner, err = usersDB.Get(tenantCtx, owner.ID)
		require.NoError(t, err)

		ctxWithOwner := console.WithUser(tenantCtx, owner)
		project, err := service.CreateProject(ctxWithOwner, console.UpsertProjectInfo{
			Name: "Test Project",
		})
		require.NoError(t, err)

		t.Run("GetInviteLink uses SingleWhiteLabel external address", func(t *testing.T) {
			// Invite user to project.
			_, err := service.InviteNewProjectMember(ctxWithOwner, project.ID, inviteeEmail)
			require.NoError(t, err)

			// Get the invite link.
			link, err := service.GetInviteLink(ctxWithOwner, project.PublicID, inviteeEmail)
			require.NoError(t, err)

			// Verify the link starts with SingleWhiteLabel's external address.
			require.True(t, len(link) > len(singleExternalAddr), "Invite link should be longer than external address")
			require.Contains(t, link, singleExternalAddr, "Invite link should use SingleWhiteLabel external address")
			require.Contains(t, link, "/invited?invite=", "Invite link should contain invite path")
		})
	})
}

// TestSingleWhiteLabelBranding verifies that SingleWhiteLabel mode returns
// the correct branding configuration.
func TestSingleWhiteLabelBranding(t *testing.T) {
	t.Run("Enabled - returns custom branding", func(t *testing.T) {
		const (
			singleTenantID     = "single-brand"
			singleTenantName   = "Single Brand Corp"
			singleSupportURL   = "https://support.example.test"
			singleDocsURL      = "https://docs.example.test"
			singleExternalAddr = "https://console.example.test/"
		)

		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1,
			Reconfigure: testplanet.Reconfigure{
				Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
					config.Console.SingleWhiteLabel = console.SingleWhiteLabelConfig{
						TenantID:        singleTenantID,
						Name:            singleTenantName,
						ExternalAddress: singleExternalAddr,
						SupportURL:      singleSupportURL,
						DocsURL:         singleDocsURL,
					}
				},
			},
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			sat := planet.Satellites[0]

			// Make request to branding endpoint.
			req, err := http.NewRequestWithContext(ctx, http.MethodGet,
				sat.ConsoleURL()+"/api/v0/config/branding", nil)
			require.NoError(t, err)
			req.Host = "localhost" // Any host should work in SingleWhiteLabel mode.

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var branding map[string]interface{}
			err = json.Unmarshal(body, &branding)
			require.NoError(t, err)

			require.Equal(t, singleTenantName, branding["name"])
			require.Equal(t, singleSupportURL, branding["supportUrl"])
			require.Equal(t, singleDocsURL, branding["docsUrl"])
		})
	})

	t.Run("Disabled - returns default Storj branding", func(t *testing.T) {
		testplanet.Run(t, testplanet.Config{
			SatelliteCount: 1,
			// No SingleWhiteLabel configured - should use default Storj branding.
		}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			sat := planet.Satellites[0]

			// Make request to branding endpoint.
			req, err := http.NewRequestWithContext(ctx, http.MethodGet,
				sat.ConsoleURL()+"/api/v0/config/branding", nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var branding map[string]interface{}
			err = json.Unmarshal(body, &branding)
			require.NoError(t, err)

			// Should return default Storj branding.
			require.Equal(t, "Storj", branding["name"])
		})
	})
}
