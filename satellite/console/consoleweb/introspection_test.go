// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb_test

import (
	"net/http"
	"os"
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
					`}))
			require.Equal(test.t, http.StatusOK, resp.StatusCode)
			_ = body

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

type file struct {
	fname    string
	fullpath string
	size     int64

	algorithm string
	checksum  string
}

func checkfile(fname string) bool { // check that the file exists and is not  directory
	status, err := os.Stat(fn) // file info will give us an exists status
	if err != nil {
		return false
	}

	if os.IsNotExist(status) { // return false when the file doesn't exist
		return false
	}
	return !status.IsDir() // fall thru and return true for not a directory
}

func openfile(relativePath string, fname string) (*file, error) {
	
	ofile := relativePath + "/" + fname
	existingFile := checkfile(ofile)

	if existingFile != true {
		return nil, nil
	}

	fhand, err := os.Open(ofile) // file handler or err
	if err != nil {
		return nil, err
	}
	defer closeFile(fhand)

	fileinfo, err := fhand.Stat()
	if err != nil {
		return nil, err
	}

	f := file{
		fname: fname,
		fullpath: ofile,
		size:	fileinfo.Size()
	} // instantiate struct file and set the properties

	return &f // return pointer to file
}

func closeFile(f *os.File) {
	err := f.Close()
	if err != nil {
		os.Exit(1)
	}
}
