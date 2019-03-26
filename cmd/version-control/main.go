// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"strings"

	"storj.io/storj/internal/version"
)

var (
	ver []version.Info
)

func handleGet(w http.ResponseWriter, r *http.Request) {
	var xfor string

	// Only handle GET Requests
	if r.Method == "GET" {
		if xfor = r.Header.Get("X-Forwarded-For"); xfor == "" {
			xfor = r.RemoteAddr
		}
		log.Printf("Request from: %s for %s", r.RemoteAddr, xfor)

		w.Header().Set("Content-Type", "application/json")
		response, err := json.Marshal(ver)
		if err != nil {
			w.WriteHeader(500)
		}

		w.WriteHeader(200)
		_, err = w.Write(response)
		if err != nil {
			log.Printf("error writing response to client: %v", err)
		}
	}
}

func main() {
	// Flags to specify required Version
	addr := flag.String("listen", "0.0.0.0:8080", "Defines Listen Address of Webserver")
	versions := flag.String("version", "v0.1.0,v0.1.1", "Comma separated list of Versions")
	flag.Parse()

	if flag.Parsed() {

		subversions := strings.Split(*versions, ",")
		for _, subversion := range subversions {
			instance := version.Info{
				Version: subversion,
			}
			ver = append(ver, instance)
		}

		log.Printf("setting version info to: %v", ver)
		http.HandleFunc("/", handleGet)
		log.Println("starting Webserver")

		// Not pretty but works..
		log.Fatal(http.ListenAndServe(*addr, nil))
	}

}
