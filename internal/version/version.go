// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

// V contains all necessary values for comparing different binary version with each other.
// It's goal is to limit the startup up outdated binaries as well as having an easier way to ensure the software
// is on the latest version from a UI/UX perspective
type V struct {
	Timestamp  string `json:"timestamp,omitempty"`
	CommitHash string `json:"commitHash,omitempty"`
	Version    string `json:"version"`
	Release    bool   `json:"release,omitempty"`
}

// New creates Version_Info from a json byte array
func New(data []byte) (v V, err error) {
	err = json.Unmarshal(data, v)
	return v, err
}

// Marshal converts the existing Version Info to any json byte array
func (v V) Marshal() (data []byte, err error) {
	data, err = json.Marshal(v)
	return
}

// QueryVersionFromControlServer handles the HTTP request to gather the allowed and latest version information
func QueryVersionFromControlServer() (ver []V, err error) {
	resp, err := http.Get("https://version.alpha.storj.io")
	if err != nil {
		return []V{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []V{}, err
	}

	err = json.Unmarshal(body, ver)
	return
}
