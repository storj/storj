// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"storj.io/storj/internal/version"
)

type runConfig struct {
	Listen   string `json:"listen"`
	Versions string `json:"versions"`
}

var (
	ver      []version.Info
	response []byte
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
		w.WriteHeader(200)
		_, err := w.Write(response)
		if err != nil {
			log.Printf("error writing response to client: %v", err)
		}
	}
}

func main() {
	var conf runConfig

	// Flags to specify required Version
	addr := flag.String("listen", "0.0.0.0:8080", "Defines Listen Address of Webserver")
	fconfig := flag.String("config-file", "", "Specifies a config file to read Versions from")
	fversions := flag.String("version", "v0.1.0,v0.1.1", "Comma separated list of Versions")

	flag.Parse()

	if !flag.Parsed() {
		log.Fatal("Error while parsing flags")
	}

	// Check for existence of Versions to use, prefer flag, fall back to config
	if *fconfig == "" && *fversions == "" {
		log.Fatal("No Versions specified, use either config file or flags to set them")
	}

	// If using config flag ensure to only accept it as flag
	if *fconfig != "" {
		file, err := os.Open(*fconfig)
		if err != nil {
			log.Fatalf("Could not open configuation file: %s", *fconfig)
		}
		bytes, err := ioutil.ReadAll(file)
		if err != nil {
			log.Fatalf("Could not read configuation file: %s", *fconfig)
		}
		err = json.Unmarshal(bytes, &conf)
		if err != nil {
			log.Fatalf("Could not parse configuation file: %s %v", *fconfig, err)
		}
	} else {
		conf.Listen = *addr
		conf.Versions = *fversions
	}

	subVersions := strings.Split(conf.Versions, ",")
	for _, subVersion := range subVersions {
		instance := version.Info{
			Version: subVersion,
		}
		ver = append(ver, instance)
	}

	var err error
	response, err = json.Marshal(ver)
	if err != nil {
		log.Fatalf("Error marshalling version info: %v", err)
		return
	}

	log.Printf("setting version info to: %v", ver)
	http.HandleFunc("/", handleGet)
	log.Printf("starting webserver on %s", conf.Listen)

	// Not pretty but works..
	log.Fatal(http.ListenAndServe(conf.Listen, nil))
}
