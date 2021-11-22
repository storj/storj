// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.
package consoleweb_test

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet" 
)

func TestIntrospection(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 0, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		test := NewTest(t, ctx, planet)

		{ // Introspection
			resp, body := test.request(http.MethodPost, "/graphql",
				test.toJSON(map[string]interface{}{
					"query": `
				query IntrospectionQuery {
					__schema {
					  queryType {
						name
					  }
					  mutationType {
						name
					  }
					  subscriptionType {
						name
					  }
					  types {
						...FullType
					  }
					  directives {
						name
						description
						args {
						  ...InputValue
						}
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
					type {
					  ...TypeRef
					}
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
				`}))
			require.Contains(t, body, test.defaultProjectID())
			require.Equal(t, http.StatusOK, resp.StatusCode)
			fmt.Println(resp.Body)
		}
	})
}
