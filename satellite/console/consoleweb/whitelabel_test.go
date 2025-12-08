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
		tenant1ID       = "tenant1"
		tenant1Hostname = "tenant1.example.com"
		tenant2ID       = "tenant2"
		tenant2Hostname = "tenant2.example.com"
		sharedEmail     = "user@example.com"
		password        = "password123"
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.WhiteLabel.Value = map[string]console.WhiteLabelConfig{
					tenant1ID: {
						TenantID: tenant1ID,
						HostName: tenant1Hostname,
						Name:     "Tenant One",
					},
					tenant2ID: {
						TenantID: tenant2ID,
						HostName: tenant2Hostname,
						Name:     "Tenant Two",
					},
				}
				config.Console.WhiteLabel.HostNameIDLookup = map[string]string{
					tenant1Hostname: tenant1ID,
					tenant2Hostname: tenant2ID,
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		usersDB := sat.DB.Console().Users()

		var (
			tenant1IDStr = tenant1ID
			tenant2IDStr = tenant2ID
		)

		t.Run("Create user on tenant1", func(t *testing.T) {
			tenantID1 := tenancy.FromHostname(tenant1Hostname, sat.Config.Console.WhiteLabel.HostNameIDLookup)
			ctx1 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenantID1})

			user1, err := service.CreateUser(ctx1, console.CreateUser{
				FullName: "User On Tenant 1",
				Email:    sharedEmail,
				Password: password,
			}, console.RegistrationSecret{})
			require.NoError(t, err)
			require.NotNil(t, user1)
			require.Equal(t, sharedEmail, user1.Email)
			require.NotNil(t, user1.TenantID)
			require.Equal(t, tenant1ID, *user1.TenantID)
		})

		t.Run("Create user with same email on tenant2", func(t *testing.T) {
			tenantID2 := tenancy.FromHostname(tenant2Hostname, sat.Config.Console.WhiteLabel.HostNameIDLookup)
			ctx2 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenantID2})

			user2, err := service.CreateUser(ctx2, console.CreateUser{
				FullName: "User On Tenant 2",
				Email:    sharedEmail,
				Password: password,
			}, console.RegistrationSecret{})
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
			}, console.RegistrationSecret{})
			require.NoError(t, err)
			require.NotNil(t, userDefault)
			require.Equal(t, sharedEmail, userDefault.Email)
			require.Nil(t, userDefault.TenantID)
		})

		t.Run("Duplicate email within same tenant fails", func(t *testing.T) {
			tenantID1 := tenancy.FromHostname(tenant1Hostname, sat.Config.Console.WhiteLabel.HostNameIDLookup)
			ctx1 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenantID1})

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
			}, console.RegistrationSecret{})
			require.Error(t, err)
			require.True(t, console.ErrEmailUsed.Has(err))
		})
	})
}

// TestAuthenticationTenantIsolation verifies that login is tenant-aware and users can only authenticate within their tenant context.
func TestAuthenticationTenantIsolation(t *testing.T) {
	const (
		tenant1ID       = "tenant1"
		tenant1Hostname = "tenant1.example.com"
		tenant2ID       = "tenant2"
		tenant2Hostname = "tenant2.example.com"
		email           = "user@example.com"
		password        = "password123"
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.WhiteLabel.Value = map[string]console.WhiteLabelConfig{
					tenant1ID: {
						TenantID: tenant1ID,
						HostName: tenant1Hostname,
						Name:     "Tenant One",
					},
					tenant2ID: {
						TenantID: tenant2ID,
						HostName: tenant2Hostname,
						Name:     "Tenant Two",
					},
				}
				config.Console.WhiteLabel.HostNameIDLookup = map[string]string{
					tenant1Hostname: tenant1ID,
					tenant2Hostname: tenant2ID,
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		usersDB := sat.DB.Console().Users()

		tenantID1 := tenancy.FromHostname(tenant1Hostname, sat.Config.Console.WhiteLabel.HostNameIDLookup)
		ctx1 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenantID1})

		user1, err := service.CreateUser(ctx1, console.CreateUser{
			FullName: "User One",
			Email:    email,
			Password: password,
		}, console.RegistrationSecret{})
		require.NoError(t, err)

		user1.Status = console.Active
		err = usersDB.Update(ctx1, user1.ID, console.UpdateUserRequest{
			Status: &user1.Status,
		})
		require.NoError(t, err)

		tenantID2 := tenancy.FromHostname(tenant2Hostname, sat.Config.Console.WhiteLabel.HostNameIDLookup)
		ctx2 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenantID2})

		user2, err := service.CreateUser(ctx2, console.CreateUser{
			FullName: "User Two",
			Email:    email,
			Password: password,
		}, console.RegistrationSecret{})
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
		tenant1ID       = "tenant1"
		tenant1Hostname = "tenant1.example.com"
		tenant2ID       = "tenant2"
		tenant2Hostname = "tenant2.example.com"
		user1Email      = "user1@example.com"
		user2Email      = "user2@example.com"
		password        = "password123"
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.WhiteLabel.Value = map[string]console.WhiteLabelConfig{
					tenant1ID: {
						TenantID: tenant1ID,
						HostName: tenant1Hostname,
						Name:     "Tenant One",
					},
					tenant2ID: {
						TenantID: tenant2ID,
						HostName: tenant2Hostname,
						Name:     "Tenant Two",
					},
				}
				config.Console.WhiteLabel.HostNameIDLookup = map[string]string{
					tenant1Hostname: tenant1ID,
					tenant2Hostname: tenant2ID,
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.API.Console.Service
		usersDB := sat.DB.Console().Users()
		projectsDB := sat.DB.Console().Projects()

		tenantID1 := tenancy.FromHostname(tenant1Hostname, sat.Config.Console.WhiteLabel.HostNameIDLookup)
		ctx1 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenantID1})

		user1, err := service.CreateUser(ctx1, console.CreateUser{
			FullName: "User One",
			Email:    user1Email,
			Password: password,
		}, console.RegistrationSecret{})
		require.NoError(t, err)

		user1.Status = console.Active
		err = usersDB.Update(ctx1, user1.ID, console.UpdateUserRequest{
			Status: &user1.Status,
		})
		require.NoError(t, err)

		tenantID2 := tenancy.FromHostname(tenant2Hostname, sat.Config.Console.WhiteLabel.HostNameIDLookup)
		ctx2 := tenancy.WithContext(ctx, &tenancy.Context{TenantID: tenantID2})

		user2, err := service.CreateUser(ctx2, console.CreateUser{
			FullName: "User Two",
			Email:    user2Email,
			Password: password,
		}, console.RegistrationSecret{})
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

// TestBrandingAPIPerTenant verifies that the branding API returns the correct configuration for each tenant.
func TestBrandingAPIPerTenant(t *testing.T) {
	const (
		tenant1ID   = "tenant1"
		tenant1Host = "tenant1.example.com"
		tenant2ID   = "tenant2"
		tenant2Host = "tenant2.example.com"
	)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Console.WhiteLabel.Value = map[string]console.WhiteLabelConfig{
					tenant1ID: {
						TenantID: tenant1ID,
						HostName: tenant1Host,
						Name:     "Tenant One Branding",
						LogoURLs: map[string]string{
							"full-dark": "https://tenant1.example.com/logo.png",
						},
						Colors: map[string]string{
							"primary": "#FF0000",
						},
						SupportURL: "https://support.tenant1.example.com",
					},
					tenant2ID: {
						TenantID: tenant2ID,
						HostName: tenant2Host,
						Name:     "Tenant Two Branding",
						LogoURLs: map[string]string{
							"full-dark": "https://tenant2.example.com/logo.png",
						},
						Colors: map[string]string{
							"primary": "#00FF00",
						},
						SupportURL: "https://support.tenant2.example.com",
					},
				}
				config.Console.WhiteLabel.HostNameIDLookup = map[string]string{
					tenant1Host: tenant1ID,
					tenant2Host: tenant2ID,
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		addr := sat.API.Console.Listener.Addr().String()
		client := http.DefaultClient

		t.Run("Tenant1 branding", func(t *testing.T) {
			url := "http://" + addr + "/api/v0/config/branding"
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
			require.NoError(t, err)

			req.Host = tenant1Host

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
			require.Equal(t, "public, max-age=3600", resp.Header.Get("Cache-Control"))

			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var branding map[string]any
			require.NoError(t, json.Unmarshal(bodyBytes, &branding))

			require.Equal(t, "Tenant One Branding", branding["name"])
			require.Equal(t, "https://support.tenant1.example.com", branding["supportUrl"])

			logoURLs, ok := branding["logoUrls"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "https://tenant1.example.com/logo.png", logoURLs["full-dark"])

			colors, ok := branding["colors"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "#FF0000", colors["primary"])
		})

		t.Run("Tenant2 branding", func(t *testing.T) {
			url := "http://" + addr + "/api/v0/config/branding"
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
			require.NoError(t, err)

			req.Host = tenant2Host

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var branding map[string]any
			require.NoError(t, json.Unmarshal(bodyBytes, &branding))

			require.Equal(t, "Tenant Two Branding", branding["name"])
			require.Equal(t, "https://support.tenant2.example.com", branding["supportUrl"])

			colors, ok := branding["colors"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "#00FF00", colors["primary"])
		})

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

		t.Run("Case insensitive hostname matching", func(t *testing.T) {
			url := "http://" + addr + "/api/v0/config/branding"
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
			require.NoError(t, err)

			req.Host = strings.ToUpper(tenant1Host)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var branding map[string]any
			require.NoError(t, json.Unmarshal(bodyBytes, &branding))

			require.NotEmpty(t, branding["name"])
		})

		t.Run("Hostname with port number", func(t *testing.T) {
			url := "http://" + addr + "/api/v0/config/branding"
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
			require.NoError(t, err)

			req.Host = tenant1Host + ":8080"

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer func() { require.NoError(t, resp.Body.Close()) }()

			require.Equal(t, http.StatusOK, resp.StatusCode)

			bodyBytes, err := io.ReadAll(resp.Body)
			require.NoError(t, err)

			var branding map[string]any
			require.NoError(t, json.Unmarshal(bodyBytes, &branding))

			require.NotEmpty(t, branding["name"])
		})
	})
}
