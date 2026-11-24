// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/testcontext"
	"storj.io/common/uuid"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/admin"
	"storj.io/storj/satellite/admin/auditlogger"
	"storj.io/storj/satellite/admin/changehistory"
)

func TestUpdateTenantWhiteLabelConfig(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(_ *zap.Logger, _ int, config *satellite.Config) {
				config.Admin.AuditLogger.Enabled = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		authInfo := &admin.AuthInfo{Email: "admin@example.com", Groups: []string{"admin"}}

		tenantID := "whitelabel-tenant"
		request := admin.UpdateTenantWhiteLabelConfigRequest{
			ConfigYAML: "name: Acme\ncolors:\n  primary-light: '#FF0000'\n",
		}

		_, apiErr := service.UpdateTenantWhiteLabelConfig(ctx, authInfo, tenantID, request)
		require.Equal(t, http.StatusBadRequest, apiErr.Status)
		require.Contains(t, apiErr.Err.Error(), "reason is required")

		request.Reason = "rebrand"
		config, apiErr := service.UpdateTenantWhiteLabelConfig(ctx, authInfo, tenantID, request)
		require.NoError(t, apiErr.Err)
		require.Equal(t, tenantID, config.TenantID)

		config, apiErr = service.GetTenantWhiteLabelConfig(ctx, tenantID)
		require.NoError(t, apiErr.Err)
		require.Contains(t, config.ConfigYAML, "name: Acme")
		require.Contains(t, config.ConfigYAML, "primary-light: '#FF0000'")

		// the change is recorded for the tenant.
		require.Eventually(t, func() bool {
			changes, err := sat.DB.AdminChangeHistory().GetChangesByUserID(ctx, uuid.UUID{}, false)
			require.NoError(t, err)
			for _, change := range changes {
				if change.Operation == "update_tenant_whitelabel_config" {
					require.Equal(t, changehistory.ItemTypeTenant, change.ItemType)
					require.Equal(t, "rebrand", change.Reason)
					require.Equal(t, "admin@example.com", change.AdminEmail)
					require.Equal(t, tenantID, change.Changes[auditlogger.ChangeSetItemIDKey])
					require.Contains(t, change.Changes, "Name")
					return true
				}
			}
			return false
		}, 5*time.Second, 50*time.Millisecond)
	})
}
