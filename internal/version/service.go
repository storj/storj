// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
)

// CheckVersion_Startup ensures that client is running latest/allowed code, else refusing further operation
func CheckVersionStartup(ctx *context.Context) (err error) {
	allow, err := CheckVersion(ctx)
	if err == nil {
		Allowed = allow
	}
	return
}

// CheckVersion checks if the client is running latest/allowed code
func CheckVersion(ctx *context.Context) (allowed bool, err error) {
	defer mon.Task()(ctx)(&err)

	accepted, err := queryVersionFromControlServer()
	if err != nil {
		return
	}

	zap.S().Debugf("allowed versions from Control Server: %v", accepted)

	if containsVersion(accepted, Build.Version) {
		zap.S().Infof("running on version %s", Build.Version.String())
		allowed = true
	} else {
		zap.S().Errorf("running on not allowed/outdated version %s", Build.Version.String())
		allowed = false
	}
	return
}

// QueryVersionFromControlServer handles the HTTP request to gather the allowed and latest version information
func queryVersionFromControlServer() (ver []Info, err error) {
	resp, err := http.Get("https://satellite.stefan-benten.de/version")
	if err != nil {
		// ToDo: Make sure Control Server is always reachable and refuse startup
		Allowed = true
		return []Info{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []Info{}, err
	}

	err = json.Unmarshal(body, &ver)
	return
}

// DebugHandler returns a json representation of the current version information for the binary
func DebugHandler(w http.ResponseWriter, r *http.Request) {
	j, err := Build.Marshal()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(j)
	if err != nil {
		zap.S().Errorf("error writing data to client %v", err)
	}
}

// containsVersion compares the allowed version array against the passed version
func containsVersion(all []Info, x SemVer) bool {
	for _, n := range all {
		if x == n.Version {
			return true
		}
	}
	return false
}
