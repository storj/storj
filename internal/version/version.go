// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"encoding/json"
	"errors"
	"fmt"
  "io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Timestamp is the UTC timestamp of the compilation time
	Timestamp string
	// CommitHash is the git hash of the code being compiled
	CommitHash string
	// Version is the semantic version set at compilation
	// if not a valid semantic version Release should be false
	Version string
	// Release indicates whether the binary compiled is a release candidate
	Release bool
	// Build is a struct containing all relevant build information associated with the binary
	Build V
	// Accepted is a list of accepted Version that we got from the control server
	Accepted []V
)

// V is the versioning information for a binary
type V struct {
	Timestamp  string `json:"timestamp,omitempty"`
	CommitHash string `json:"commitHash,omitempty"`
	Version    SemVer `json:"version"`
	Release    bool   `json:"release,omitempty"`
}

// SemVer represents a semantic version
type SemVer struct {
	Major int64 `json:"major"`
	Minor int64 `json:"minor"`
	Patch int64 `json:"patch"`
}

// SemVerRegex is the regular expression used to parse a semantic version.
// https://github.com/Masterminds/semver/blob/master/LICENSE.txt
const SemVerRegex string = `v?([0-9]+)(\.[0-9]+)?(\.[0-9]+)?` +
	`(-([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?` +
	`(\+([0-9A-Za-z\-]+(\.[0-9A-Za-z\-]+)*))?`

func init() {
	Build = V{
		Timestamp:  Timestamp,
		CommitHash: CommitHash,
		Release:    Release,
	}

	versionRegex := regexp.MustCompile("^" + SemVerRegex + "$")

	sv, err := NewSemVer(versionRegex, Version)
	if err != nil {
		panic(err)
	}

	Build.Version = *sv

	if Build.Timestamp == "" || Build.CommitHash == "" {
		Build.Release = false
	}
}

// Handler returns a json representation of the current version information for the binary
func Handler(w http.ResponseWriter, r *http.Request) {
	j, err := json.Marshal(Build)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(j)
	if err != nil {
		// ToDo: Handle Error
	}
}

// NewSemVer parses a given version and returns an instance of SemVer or
// an error if unable to parse the version.
func NewSemVer(regex *regexp.Regexp, v string) (*SemVer, error) {
	m := regex.FindStringSubmatch(v)
	if m == nil {
		return nil, errors.New("invalid semantic version for build")
	}

	sv := SemVer{}

	var err error
	sv.Major, err = parse(m[0])
	if err != nil {
		return nil, err
	}

	if m[2] == "" {
		sv.Minor = 0
	} else {
		sv.Minor, err = parse(m[2])
		if err != nil {
			return nil, err
		}
	}

	if m[3] == "" {
		sv.Patch = 0
	} else {
		sv.Minor, err = parse(m[3])
		if err != nil {
			return nil, err
		}
	}

	return &sv, nil
}

// String converts the SemVer struct to a more easy to handle string
func (sem *SemVer) String() (version string) {
	return fmt.Sprintf("v%d.%d.%d", sem.Major, sem.Minor, sem.Patch)
}

// parse converts a string with schema .xxx to an int64 or returns an error
func parse(label string) (int64, error) {
	l, err := strconv.ParseInt(strings.TrimPrefix(label, "."), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid semantic version: %s", err)
	}

	return l, nil	

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
