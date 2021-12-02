// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
)

func TestIntrospection(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		test := newTest(t, ctx, planet)
		
		{ // Get Schema via Introspection
			resp, body := test.request(
				http.MethodPost, "/graphql",
				test.toJSON(map[string]interface{}{
					"query": `
					query IntrospectionQuery {
						__schema {
						  queryType { name }
						  mutationType { name }
						  subscriptionType { name }
						  types {
							...FullType
						  }
						  directives {
							name
							description
							locations
							args {
							  ...InputValue
							}
							# deprecated, but included for coverage till removed
							onOperation
							onFragment
							onField
						  }
						}
					  }
					
					  fragment FullType on __Type {
						kind
						name
						description
						fields(includeDeprecated: true) {
						  name
						  description
						  args {
							...InputValue
						  }
						  type {
							...TypeRef
						  }
						  isDeprecated
						  deprecationReason
						}
						inputFields {
						  ...InputValue
						}
						interfaces {
						  ...TypeRef
						}
						enumValues(includeDeprecated: true) {
						  name
						  description
						  isDeprecated
						  deprecationReason
						}
						possibleTypes {
						  ...TypeRef
						}
					  }
					
					  fragment InputValue on __InputValue {
						name
						description
						type { ...TypeRef }
						defaultValue
					  }
					
					  fragment TypeRef on __Type {
						kind
						name
						ofType {
						  kind
						  name
						  ofType {
							kind
							name
							ofType {
							  kind
							  name
							  ofType {
								kind
								name
								ofType {
								  kind
								  name
								  ofType {
									kind
									name
									ofType {
									  kind
									  name
									}
								  }
								}
							  }
							}
						  }
						}
					  }
					}`,}))

			fmt.Println(resp)
			fmt.Println(body)
		}
	})
}

func TestAuthAttempts(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		test := newTest(t, ctx, planet)
		
		{ // repeated login attempts should end in too many requests
			hitRateLimiter := false
			for i := 0; i < 30; i++ {
				resp, _ := test.request(
					http.MethodPost, "/auth/token",
					strings.NewReader(`{"email":"wrong@invalid.test","password":"wrong"}`))
				require.Nil(t, findCookie(resp, "_tokenKey"))
				if resp.StatusCode != http.StatusUnauthorized {
					require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
					hitRateLimiter = true
					break
				}
			}
			require.True(t, hitRateLimiter, "did not hit rate limiter")
		}
	})
}
