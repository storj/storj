package main

import  (
	"fmt"
//	"flag"
)

type httpHeaders struct {
	X_API_KEY [] string
	Connection [] string 
	Cache_Control [] string
	Upgrade_Insecure_Requests [] string 
	Accept [] string
	Accept_Language [] string
}

func main() {
	httpRequestHeaders := createHeaderStruct()
	validateApiKeyWithEnv(httpRequestHeaders)
	validateApiKeyWithFlags(httpRequestHeaders)
}

func createHeaderStruct() *httpHeaders {
	// mock HTTP request headers
	// client can set headers x-api-key with 
	//req.Header.Add("x-api-key", "apikey")

	requestHeader := httpHeaders {
		X_API_KEY: []string{"12345"},
		Connection: []string{"keep-alive"},
		Cache_Control: []string{"max-age=0"},
		Upgrade_Insecure_Requests: []string{"1"},
		Accept: []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8" },
		Accept_Language: []string{"en-US,en;q=0.9"},
	}
	return &requestHeader
}

func validateApiKeyWithEnv(headers* httpHeaders) {
	
	fmt.Println(headers)
}

func validateApiKeyWithFlags(headers* httpHeaders) {
	
	fmt.Println(headers)
}
