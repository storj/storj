// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package auth

import  (
	"os"
	"net/http"
	"crypto/subtle"
	
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
	// desgined to match xApiKey

	viper.SetEnvPrefix("API") 
	os.Setenv("API_KEY", "12345")
	viper.AutomaticEnv()
}

func ValidateApiKey(header string)(bool) {
	// validates apikey with xApiKey header

	apiKey := viper.GetString("key")

	if len(apiKey) == 0 {
		setEnv()
	}

	var apiKeyByte []byte = []byte(viper.GetString("key"))
	var xApiKeyByte []byte = []byte(header)
	
	switch  {		
		case len(apiKeyByte) == 0:
			return false
		case len(apiKeyByte) > 0:
			result := subtle.ConstantTimeCompare(apiKeyByte, xApiKeyByte)
			if result > 0 {
				return true
			} else {
				return false
			}	 
		 }
	return false
}
