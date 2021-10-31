//  Copyright (C) 2021 Storj Labs, Inc.
//  See LICENSE for copying information.

package endpoints

// delete the graphql_schema.txt if the endpoints were modified.
// a new one with updated data will automatically be created.
import (
	"os"
)

var (
	req      []byte
	satenv   string
	saturl   = "/api/v0/graphql"
	exitcode = 0
)

// Endpoints gets all endpoints and compairs them to a known list of possible endpoints.
func Endpoints() int {

	// build the satellite url from the environment variable.
	satenv = os.Getenv("SATELLITE_0_ADDR")
	saturl = "http:// " + satenv + saturl
	req = introspect(saturl) // call introspect from handler.go.

	return exitcode
}
