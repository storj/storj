// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"

	"storj.io/storj/netstate/auth"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// example of how the auth package is working.
// see readme in auth/ for how to run
func main() {

	pflag.String("key", "", "this is your API KEY")
	viper.BindPFlag("key", pflag.Lookup("key"))
	pflag.Parse()

	viper.SetEnvPrefix("API")
	os.Setenv("API_KEY", "12345")
	viper.AutomaticEnv()

	httpRequestHeaders := auth.InitializeHeaders()
	xAPIKey := httpRequestHeaders.Get("X-Api-Key")

	isAuthorized := auth.ValidateAPIKey(xAPIKey)
	fmt.Println(isAuthorized)
}
