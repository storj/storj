package version

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

// Info contains all necessary values for comparing different binary version with each other.
// It's goal is to limit the startup up outdated binaries as well as having an easier way to ensure the software
// is on the latest version from a UI/UX perspective
type Info struct {
	Timestamp  string `json:"timestamp,omitempty"`
	CommitHash string `json:"hash,omitempty"`
	Version    string `json:"version"`
	Release    bool   `json:"release,omitempty"`
}

// New creates Version_Info from a json byte array
func New(data []byte) (v Info, err error) {
	err = json.Unmarshal(data, v)
	return v, err
}

// Marshal converts the existing Version Info to any json byte array
func (v Info) Marshal() (data []byte, err error) {
	data, err = json.Marshal(v)
	return
}

// QueryVersionFromControlServer handles the HTTP request to gather the allowed and latest version information
func QueryVersionFromControlServer() (ver []Info, err error) {
	resp, err := http.Get("https://version.satellite.storj.io")
	if err != nil {
		return []Info{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []Info{}, err
	}

	err = json.Unmarshal(body, ver)
	return
}
