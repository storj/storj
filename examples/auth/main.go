// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	//"storj.io/storj/netstate/auth"
)

// example of how the auth package is working.
// see readme in auth/ for how to run
func main() {

	// set client api credentials
	// hardcoded as an example
	pflag.String("key", "", "this is your API KEY")
	viper.BindPFlag("key", pflag.Lookup("key"))
	pflag.Parse()

	viper.SetEnvPrefix("API")
	os.Setenv("API_KEY", "abc123")
	viper.AutomaticEnv()

	// json string we will include in the PUT request
	var jsonString = []byte(`{"value":"hello world"}`)

	client := &http.Client{}
	req, err := http.NewRequest("PUT", "http://localhost:3000/file/my/test/file", bytes.NewBuffer(jsonString))
	req.Header.Add("X-Api-Key", viper.GetString("key"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)

	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Println("the rsponse is: ", resp)
}
