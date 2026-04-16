// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package admin_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/admin"
)

func TestDisqualifyNode(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		service.TestSetBypassAuth(true)

		nodeID := planet.StorageNodes[0].ID().String()
		authInfo := &admin.AuthInfo{Email: "admin@example.com", Groups: []string{"admin"}}

		t.Run("requires auth", func(t *testing.T) {
			apiErr := service.DisqualifyNode(ctx, nil, nodeID, admin.DisqualifyNodeRequest{Reason: "test"})
			require.Equal(t, http.StatusUnauthorized, apiErr.Status)
		})

		t.Run("requires reason", func(t *testing.T) {
			apiErr := service.DisqualifyNode(ctx, authInfo, nodeID, admin.DisqualifyNodeRequest{})
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})

		t.Run("rejects invalid node ID", func(t *testing.T) {
			apiErr := service.DisqualifyNode(ctx, authInfo, "invalid-id", admin.DisqualifyNodeRequest{Reason: "test"})
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})

		t.Run("rejects unknown node ID", func(t *testing.T) {
			unknownID := testrand.NodeID().String()
			apiErr := service.DisqualifyNode(ctx, authInfo, unknownID, admin.DisqualifyNodeRequest{Reason: "test"})
			require.Equal(t, http.StatusNotFound, apiErr.Status)
		})

		t.Run("rejects tenant-scoped admin", func(t *testing.T) {
			tenantA := "tenant-a"
			service.TestSetTenantID(&tenantA)
			defer service.TestSetTenantID(nil)

			apiErr := service.DisqualifyNode(ctx, authInfo, nodeID, admin.DisqualifyNodeRequest{Reason: "test"})
			require.Equal(t, http.StatusForbidden, apiErr.Status)
		})

		t.Run("disqualifies node successfully", func(t *testing.T) {
			apiErr := service.DisqualifyNode(ctx, authInfo, nodeID, admin.DisqualifyNodeRequest{
				Reason:                 "bad actor",
				DisqualificationReason: "audit_failure",
			})
			require.NoError(t, apiErr.Err)

			info, apiErr := service.GetNodeInfo(ctx, nodeID)
			require.NoError(t, apiErr.Err)
			require.NotNil(t, info.Disqualified)
			require.NotNil(t, info.DisqualificationReason)
			require.Equal(t, "Audit Failure", *info.DisqualificationReason)
		})

		t.Run("conflicts if already disqualified", func(t *testing.T) {
			apiErr := service.DisqualifyNode(ctx, authInfo, nodeID, admin.DisqualifyNodeRequest{Reason: "again"})
			require.Equal(t, http.StatusConflict, apiErr.Status)
		})
	})
}

func TestUndisqualifyNode(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		service := sat.Admin.Admin.Service
		service.TestSetBypassAuth(true)

		nodeID := planet.StorageNodes[0].ID().String()
		authInfo := &admin.AuthInfo{Email: "admin@example.com", Groups: []string{"admin"}}

		t.Run("requires auth", func(t *testing.T) {
			apiErr := service.UndisqualifyNode(ctx, nil, nodeID, admin.UndisqualifyNodeRequest{Reason: "test"})
			require.Equal(t, http.StatusUnauthorized, apiErr.Status)
		})

		t.Run("requires reason", func(t *testing.T) {
			apiErr := service.UndisqualifyNode(ctx, authInfo, nodeID, admin.UndisqualifyNodeRequest{})
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})

		t.Run("rejects invalid node ID", func(t *testing.T) {
			apiErr := service.UndisqualifyNode(ctx, authInfo, "invalid-id", admin.UndisqualifyNodeRequest{Reason: "test"})
			require.Equal(t, http.StatusBadRequest, apiErr.Status)
		})

		t.Run("conflicts if not disqualified", func(t *testing.T) {
			apiErr := service.UndisqualifyNode(ctx, authInfo, nodeID, admin.UndisqualifyNodeRequest{Reason: "test"})
			require.Equal(t, http.StatusConflict, apiErr.Status)
		})

		t.Run("rejects tenant-scoped admin", func(t *testing.T) {
			tenantA := "tenant-a"
			service.TestSetTenantID(&tenantA)
			defer service.TestSetTenantID(nil)

			apiErr := service.UndisqualifyNode(ctx, authInfo, nodeID, admin.UndisqualifyNodeRequest{Reason: "test"})
			require.Equal(t, http.StatusForbidden, apiErr.Status)
		})

		t.Run("undisqualifies a disqualified node", func(t *testing.T) {
			// first disqualify the node
			apiErr := service.DisqualifyNode(ctx, authInfo, nodeID, admin.DisqualifyNodeRequest{Reason: "setup", DisqualificationReason: "suspension"})
			require.NoError(t, apiErr.Err)

			apiErr = service.UndisqualifyNode(ctx, authInfo, nodeID, admin.UndisqualifyNodeRequest{Reason: "reinstated"})
			require.NoError(t, apiErr.Err)

			info, apiErr := service.GetNodeInfo(ctx, nodeID)
			require.NoError(t, apiErr.Err)
			require.Nil(t, info.Disqualified)
		})
	})
}
