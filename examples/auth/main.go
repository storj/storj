// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import  (
	"fmt"
	
	"storj.io/storj/netstate/auth"
	
	"github.com/spf13/viper"
	"github.com/spf13/pflag"
)

func main() {
	pflag.String("key", "", "this is your API KEY")
	viper.BindPFlag("key", pflag.Lookup("key"))
	pflag.Parse()
	
	httpRequestHeaders := auth.InitializeHeaders()
	xApiKey := httpRequestHeaders.Get("X-Api-Key")

	isAuthorized := auth.ValidateApiKey(xApiKey)
	fmt.Println(isAuthorized)
}
