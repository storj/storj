// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import  (
	"fmt"
	"os"
	"net/http"
	
	"github.com/spf13/viper"
)

func InitializeHeaders() *http.Header {
	// mock HTTP request headers

	httpHeaders := http.Header {
		"Accept-Encoding": {"gzip, deflate"},
		"Accept-Language": {"en-US,en;q=0.9"},
		"X-Api-Key":         {"12345"},
		"Cache-Control":    {"max-age=0"},
		"Accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8"} ,
		"Connection":      {"keep-alive"},
	}
	return &httpHeaders
}

func setEnv() {
	// if flag is not set, we'll set the env
	viper.SetEnvPrefix("API") 
	os.Setenv("API_KEY", "12345")
	viper.AutomaticEnv()
}

func ValidateApiKey(header string)(bool) {
	// validates env key with apikey header
		
	apiKey := viper.GetString("key")

	if len(apiKey) == 0 {
		setEnv()
	}

	apiKey = viper.GetString("key")

	switch {		
	  case len(apiKey) == 0:
		  return false
	  case apiKey != header:
		  return false
	  }
	  
	return true
}
