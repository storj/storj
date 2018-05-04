package main

import  (
	"fmt"
	"os"
//	"flag"
)

type httpHeaders struct {
	X_API_KEY string
	Connection string 
	Cache_Control string
	Upgrade_Insecure_Requests string 
	Accept string
	Accept_Language string
}

func main() {
	httpRequestHeaders := createHeaderStruct()
	_setEnv()
	validateApiKeyWithEnv(httpRequestHeaders)
	validateApiKeyWithFlags(httpRequestHeaders)
	
}

func createHeaderStruct() *httpHeaders {
	// mock HTTP request headers
	// client can set headers x-api-key with 
	//req.Header.Add("x-api-key", "apikey")
	requestHeader := httpHeaders {
		X_API_KEY: "12345",
		Connection: "keep-alive",
		Cache_Control: "max-age=0",
		Upgrade_Insecure_Requests: "1",
		Accept: "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8" ,
		Accept_Language: "en-US,en;q=0.9",
	}
	return &requestHeader
}

func _setEnv() {
	os.Setenv("APIKEY","1245")
}

func validateApiKeyWithEnv(headers* httpHeaders)(bool) {
	envApiKey := os.Getenv("APIKEY")
	switch {		
	case len(envApiKey) == 0:
		return false
	case envApiKey != headers.X_API_KEY:
		return false
	}
	return true
}


func validateApiKeyWithFlags(headers* httpHeaders) {
	
	//fmt.Println(headers)
}
