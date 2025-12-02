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
	backoffice "storj.io/storj/satellite/admin/back-office"
)

func TestGetSettings(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				config.Admin.Address = "127.0.0.1:0"
				config.Admin.BackOffice.UserGroupsRoleAdmin = []string{"admin"}
				config.Admin.BackOffice.UserGroupsRoleViewer = []string{"viewer"}
				config.PendingDeleteCleanup.Enabled = true
				config.PendingDeleteCleanup.User.Enabled = true
				config.PendingDeleteCleanup.Project.Enabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		adminSettings := backoffice.Settings{
			Admin: backoffice.SettingsAdmin{
				Features: backoffice.FeatureFlags{
					Account: backoffice.AccountFlags{
						CreateRestKey:       true,
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
						History:                true,
					},
					Bucket: backoffice.BucketFlags{
						List:                   true,
						View:                   true,
						UpdatePlacement:        true,
						UpdateValueAttribution: true,
						History:                true,
					},
				},
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
								View:     true,
								Search:   true,
								Projects: true,
								History:  true,
							},
							Project: backoffice.ProjectFlags{
								View:    true,
								History: true,
							},
							Bucket: backoffice.BucketFlags{
								List:    true,
								View:    true,
								History: true,
							},
						},
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
		settingsUrl := "http://" + address.String() + "/back-office/api/v1/settings/"
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
