// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"storj.io/storj/pkg/pointerdb/auth"
)

// example of how the auth package is working.
// see readme in auth/ for how to run
func main() {
	pflag.String("key", "", "this is your API KEY")
	err := viper.BindPFlag("key", pflag.Lookup("key"))
	if err != nil {
		fmt.Println(err)
	}
	pflag.Parse()

	viper.SetEnvPrefix("API")
	err = os.Setenv("API_KEY", "12345")
	if err != nil {
		fmt.Println(err)
	}
	viper.AutomaticEnv()

	httpRequestHeaders := InitializeHeaders()
	xAPIKey := httpRequestHeaders.Get("X-Api-Key")

	isAuthorized := auth.ValidateAPIKey(xAPIKey)
	fmt.Println(isAuthorized)
}

// InitializeHeaders mocks HTTP headers to help test X-API-Key
func InitializeHeaders() *http.Header {
	httpHeaders := http.Header{
		"Accept-Encoding": {"gzip, deflate"},
		"Accept-Language": {"en-US,en;q=0.9"},
		"X-Api-Key":       {"12345"},
		"Cache-Control":   {"max-age=0"},
		"Accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8"},
		"Connection":      {"keep-alive"},
	}
	return &httpHeaders
}
