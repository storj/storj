package main

import  (
	"fmt"
	"os"
	"flag"
)
 
type httpHeaders struct {
	XApiKey string
	Connection string 
	CacheControl string
	UpgradeInsecureRequests string 
	Accept string
	AcceptLanguage string
}

func main() {
	key := flag.String("key", "", "this is your API KEY")
	flag.Parse()
	
	httpRequestHeaders := createHeaderStruct()
	if len(*key) == 0 {
		_setEnv()
		validateApiKeyWithEnv(httpRequestHeaders)
	} else {
		validateApiKeyWithFlags(httpRequestHeaders, key)
	}
}

func createHeaderStruct() *httpHeaders {
	// mock HTTP request headers
	// client can set headers x-api-key with 
	//req.Header.Add("x-api-key", "apikey")
	requestHeader := httpHeaders {
		XApiKey: "12345",
		Connection: "keep-alive",
		CacheControl: "max-age=0",
		UpgradeInsecureRequests: "1",
		Accept: "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8" ,
		AcceptLanguage: "en-US,en;q=0.9",
	}
	return &requestHeader
}

func _setEnv() {
	os.Setenv("APIKEY","1245")
}

func validateApiKeyWithEnv(headers* httpHeaders)(bool) {
	// validates env key with apikey header
	
	envApiKey := os.Getenv("APIKEY")
	switch {		
	case len(envApiKey) == 0:
		return false
	case envApiKey != headers.XApiKey:
		return false
	}
	return true
}

func validateApiKeyWithFlags(headers* httpHeaders, key *string)(bool) {
	// validates flag with apikey header

	if headers.XApiKey == *key {
		return true
	} else {
		return false
	}
}
