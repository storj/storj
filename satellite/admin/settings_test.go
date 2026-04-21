// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

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
	backoffice "storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/console"
)

func TestGetSettingsBranding(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
				config.Admin.UserGroupsRoleAdmin = []string{"admin"}
				config.Console.SingleWhiteLabel = console.SingleWhiteLabelConfig{
					Name:     "AcmeCorp",
					TenantID: "acme-tenant",
					LogoURLs: map[string]string{
						"full-light": "https://acme.example.com/logo.svg",
						"full-dark":  "https://acme.example.com/logo-dark.svg",
					},
					FaviconURLs: map[string]string{
						"16x16":       "https://acme.example.com/favicon-16x16.png",
						"32x32":       "https://acme.example.com/favicon-32x32.png",
						"apple-touch": "https://acme.example.com/apple-touch-icon.png",
					},
					Colors: map[string]string{
						"primary-light": "#FF0000",
						"primary-dark":  "#CC0000",
					},
				}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		address := sat.Admin.Admin.Listener.Addr()
		settingsURL := "http://" + address.String() + "/api/v1/settings/"
		sat.Admin.Admin.Service.TestSetAllowedHost(address.String())

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, settingsURL, nil)
		require.NoError(t, err)
		req.Header.Add("X-Forwarded-Groups", "admin")
		req.Header.Add("X-Forwarded-Email", "test@example.com")

		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() { require.NoError(t, res.Body.Close()) }()

		require.Equal(t, http.StatusOK, res.StatusCode)

		resBody, err := io.ReadAll(res.Body)
		require.NoError(t, err)

		var settings backoffice.Settings
		require.NoError(t, json.Unmarshal(resBody, &settings))

		require.NotNil(t, settings.Admin.Branding)
		require.Equal(t, "AcmeCorp", settings.Admin.Branding.Name)
		require.Equal(t, "https://acme.example.com/logo.svg", settings.Admin.Branding.LogoURLs["full-light"])
		require.Equal(t, "https://acme.example.com/favicon-16x16.png", settings.Admin.Branding.FaviconURLs["16x16"])
		require.Equal(t, "#FF0000", settings.Admin.Branding.Colors["primary-light"])
		require.Equal(t, "#CC0000", settings.Admin.Branding.Colors["primary-dark"])
	})
}

func TestGetSettings_HideFreezeActions(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.UserGroupsRoleAdmin = []string{"admin"}
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		service := planet.Satellites[0].Admin.Admin.Service
		authInfo := &backoffice.AuthInfo{Groups: []string{"admin"}, Email: "test@example.com"}

		t.Run("freeze actions visible by default", func(t *testing.T) {
			service.TestSetHideFreezeActions(false)
			settings, apiErr := service.GetSettings(ctx, authInfo)
			require.NoError(t, apiErr.Err)
			require.True(t, settings.Admin.Features.Account.Suspend)
			require.True(t, settings.Admin.Features.Account.Unsuspend)
		})

		t.Run("freeze actions hidden when flag is set", func(t *testing.T) {
			service.TestSetHideFreezeActions(true)
			defer service.TestSetHideFreezeActions(false)
			settings, apiErr := service.GetSettings(ctx, authInfo)
			require.NoError(t, apiErr.Err)
			require.False(t, settings.Admin.Features.Account.Suspend)
			require.False(t, settings.Admin.Features.Account.Unsuspend)
		})

		t.Run("tenant-scoped admin has licenses and tenant-id disabled", func(t *testing.T) {
			tenantA := "tenant-a"
			service.TestSetTenantID(&tenantA)
			defer service.TestSetTenantID(nil)

			settings, apiErr := service.GetSettings(ctx, authInfo)
			require.NoError(t, apiErr.Err)
			require.False(t, settings.Admin.Features.Account.ViewLicenses)
			require.False(t, settings.Admin.Features.Account.ChangeLicenses)
			require.False(t, settings.Admin.Features.Account.UpdateTenantID)
			require.False(t, settings.Admin.Features.Bucket.History)
		})
	})
}

func TestGetSettings(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
				config.Admin.UserGroupsRoleAdmin = []string{"admin"}
				config.Admin.UserGroupsRoleViewer = []string{"viewer"}
				config.PendingDeleteCleanup.Enabled = true
				config.PendingDeleteCleanup.User.Enabled = true
				config.PendingDeleteCleanup.Project.Enabled = true
				config.Console.TenantIDList = []string{"some-tenant"}
				config.Console.ExternalAddress = "http://example.com"
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		adminSettings := backoffice.Settings{
			Admin: backoffice.SettingsAdmin{
				Features: backoffice.FeatureFlags{
					Account: backoffice.AccountFlags{
						CreateRestKey:       true,
						CreateRegToken:      true,
						Delete:              true,
						MarkPendingDeletion: true,
						DisableMFA:          true,
						View:                true,
						Search:              true,
						Projects:            true,
						Suspend:             true,
						Unsuspend:           true,
						UpdateKind:          true,
						UpdateName:          true,
						UpdateEmail:         true,
						UpdateStatus:        true,
						UpdateLimits:        true,
						UpdateUserAgent:     true,
						History:             true,
						UpdatePlacement:     true,
						UpdateUpgradeTime:   true,
						UpdateTenantID:      true,
						ViewLicenses:        true,
						ChangeLicenses:      true,
					},
					Project: backoffice.ProjectFlags{
						View:                   true,
						UpdateInfo:             true,
						UpdateLimits:           true,
						UpdatePlacement:        true,
						UpdateValueAttribution: true,
						SetEntitlements:        true,
						Delete:                 true,
						MarkPendingDeletion:    true,
						MemberList:             true,
						History:                true,
					},
					Bucket: backoffice.BucketFlags{
						List:                   true,
						View:                   true,
						UpdatePlacement:        true,
						UpdateValueAttribution: true,
						History:                true,
					},
					Access: backoffice.AccessFlags{
						Inspect: true,
					},
				},
			},
			Console: backoffice.SettingsConsole{
				ExternalAddress: "http://example.com",
				TenantIDList:    []string{"some-tenant"},
			},
		}

		type groupTest struct {
			Name       string
			groups     []string
			bypassAuth bool
			expected   *backoffice.Settings // nil means unauthorized
		}
		testCases := []groupTest{
			{
				Name:   "no groups",
				groups: []string{},
			},
			{
				Name:     "admin group",
				groups:   []string{"admin"},
				expected: &adminSettings,
			},
			{
				Name:     "admin and viewer group",
				groups:   []string{"admin", "viewer"},
				expected: &adminSettings,
			},
			{
				Name:   "viewer group",
				groups: []string{"viewer"},
				expected: &backoffice.Settings{
					Admin: backoffice.SettingsAdmin{
						Features: backoffice.FeatureFlags{
							Account: backoffice.AccountFlags{
								View:         true,
								Search:       true,
								Projects:     true,
								History:      true,
								ViewLicenses: true,
							},
							Project: backoffice.ProjectFlags{
								View:       true,
								History:    true,
								MemberList: true,
							},
							Bucket: backoffice.BucketFlags{
								List:    true,
								View:    true,
								History: true,
							},
						},
					},
					Console: backoffice.SettingsConsole{
						ExternalAddress: "http://example.com",
						TenantIDList:    []string{"some-tenant"},
					},
				},
			},
			{
				Name:       "bypass auth",
				bypassAuth: true,
				expected:   &adminSettings,
			},
		}

		sat := planet.Satellites[0]
		address := sat.Admin.Admin.Listener.Addr()
		settingsUrl := "http://" + address.String() + "/api/v1/settings/"
		sat.Admin.Admin.Service.TestSetAllowedHost(address.String())

		for _, tc := range testCases {
			t.Run(tc.Name, func(t *testing.T) {
				if tc.bypassAuth {
					sat.Admin.Admin.Service.TestSetBypassAuth(tc.bypassAuth)
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, settingsUrl, nil)
				require.NoError(t, err)
				req.Header.Add("X-Forwarded-Groups", strings.Join(tc.groups, ","))
				req.Header.Add("X-Forwarded-Email", "test@example.com")

				var res *http.Response
				defer func() {
					ctx.Check(res.Body.Close)
					if tc.bypassAuth {
						sat.Admin.Admin.Service.TestSetBypassAuth(false)
					}
				}()

				res, err = http.DefaultClient.Do(req) //nolint:bodyclose
				require.NoError(t, err)
				if tc.expected == nil {
					require.Equal(t, http.StatusUnauthorized, res.StatusCode, "response status code")
					return
				}
				require.Equal(t, http.StatusOK, res.StatusCode, "response status code")

				resBody, err := io.ReadAll(res.Body)
				require.NoError(t, err)

				// parse response body to settings
				var respSettings backoffice.Settings
				err = json.Unmarshal(resBody, &respSettings)
				require.NoError(t, err)
				require.Equal(t, *tc.expected, respSettings)
			})
		}

	})
}
