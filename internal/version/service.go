// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"context"
	"encoding/json"
	"github.com/zeebo/errs"
	"io/ioutil"
	"net/http"
	"regexp"
	"time"

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
	zap.S().Debug("Checking Versions")
	accepted, err := queryVersionFromControlServer("https://satellite.stefan-benten.de/version")
	if err != nil {
		return
	}

	zap.S().Debugf("allowed versions from Control Server: %v", accepted)

	// ToDo: Fetch own Service Tag to compare correctly!
	list := accepted.Storagenode
	if list == nil {
		return true, errs.New("Empty List from Versioning Server")
	}
	if containsVersion(list, Build.Version) {
		zap.S().Infof("running on version %s", Build.Version.String())
		allowed = true
	} else {
		zap.S().Errorf("running on not allowed/outdated version %s", Build.Version.String())
		allowed = false
	}
	return
}

// QueryVersionFromControlServer handles the HTTP request to gather the allowed and latest version information
func queryVersionFromControlServer(server string) (ver Versions, err error) {
	client := http.Client{
		Timeout: time.Duration(time.Second * 5),
	}
	resp, err := client.Get(server)
	if err != nil {
		// ToDo: Make sure Control Server is always reachable and refuse startup
		Allowed = true
		return Versions{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return Versions{}, err
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
func containsVersion(all []SemVer, x SemVer) bool {
	for _, n := range all {
		if x == n {
			return true
		}
	}
	return false
}

// StrListToSemVerList converts a list of versions to a list of SemVer
func StrListToSemVerList(serviceverisons []string) (versions []SemVer, err error) {

	versionRegex := regexp.MustCompile("^" + SemVerRegex + "$")

	for _, subversion := range serviceverisons {
		sVer, err := NewSemVer(versionRegex, subversion)
		if err != nil {
			return nil, err
		}
		versions = append(versions, *sVer)
	}
	return versions, err
}
