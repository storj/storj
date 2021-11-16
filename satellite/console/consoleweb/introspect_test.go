// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
)

func TestIntrospecion(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		test := NewTest(t, ctx, planet)

		{ //Introspection
			resp, body := test.request(http.MethodPost, "/graphql",
				test.toJSON(map[string]interface{}{
					"query": `
						{
							myProjects {
								name
								id
								description
								createdAt
								ownerId
								__typename
							}
						}`}))
			require.Contains(t, body, test.defaultProjectID())
			require.Equal(t, http.StatusOK, resp.StatusCode)
		}
	})
}
