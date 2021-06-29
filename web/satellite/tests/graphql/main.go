package main

import (
	"fmt"
	"os"

	"storj.io/storj/web/satellite/tests/graphql/endpoints"
)

func main() {
	exitcode := endpoints.Endpoints()
	fmt.Println(exitcode)
	os.Exit(exitcode)
}
