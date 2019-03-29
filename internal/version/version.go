// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	mon = monkit.Package()
	// Timestamp is the UTC timestamp of the compilation time
	Timestamp string
	// CommitHash is the git hash of the code being compiled
	CommitHash string
	// Version is the semantic version set at compilation
	// if not a valid semantic version Release should be false
	Version = "0.1.0"
	// Release indicates whether the binary compiled is a release candidate
	Release bool
	// Build is a struct containing all relevant build information associated with the binary
	Build V
	// Allowed ensures, the client is still on the allowed versions returned by the control server
	Allowed bool
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

// NewSemVer parses a given version and returns an instance of SemVer or
// an error if unable to parse the version.
func NewSemVer(regex *regexp.Regexp, v string) (*SemVer, error) {
	m := regex.FindStringSubmatch(v)
	if m == nil {
		return nil, errors.New("invalid semantic version for build")
	}

	sv := SemVer{}

	var err error

	// first entry of m is the entire version string
	sv.Major, err = parseToInt64(m[1])
	if err != nil {
		return nil, err
	}

	if m[2] == "" {
		sv.Minor = 0
	} else {
		sv.Minor, err = parseToInt64(m[2])
		if err != nil {
			return nil, err
		}
	}

	if m[3] == "" {
		sv.Patch = 0
	} else {
		sv.Patch, err = parseToInt64(m[3])
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

// New creates Version_Info from a json byte array
func New(data []byte) (v V, err error) {
	err = json.Unmarshal(data, &v)
	return v, err
}

// Marshal converts the existing Version Info to any json byte array
func (v V) Marshal() (data []byte, err error) {
	data, err = json.Marshal(v)
	return
}

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

	zap.S().Debugf("Allowed Version from Control Server: %v", accepted)

	if containsVersion(accepted, Build.Version) {
		zap.S().Infof("Running on Version %s", Build.Version.String())
		allowed = true
	} else {
		zap.S().Errorf("Running on not allowed/outdated Version %s", Build.Version.String())
		allowed = false
	}
	return
}

// QueryVersionFromControlServer handles the HTTP request to gather the allowed and latest version information
func queryVersionFromControlServer() (ver []V, err error) {
	resp, err := http.Get("https://satellite.stefan-benten.de/version")
	if err != nil {

		//ToDo: Handle Failures properly!
		Allowed = true
		return []V{}, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []V{}, err
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
		// ToDo: Handle Error
	}
}

// parseToInt64 converts a string with schema .xxx to an int64 or returns an error
func parseToInt64(label string) (int64, error) {
	tmp := strings.TrimPrefix(label, ".")
	l, err := strconv.ParseInt(tmp, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid semantic version: %s", err)
	}

	return l, nil
}

// containsVersion compares the allowed version array against the passed version
func containsVersion(a []V, x SemVer) bool {
	for _, n := range a {
		if x == n.Version {
			return true
		}
	}
	return false
}

func init() {
	if Version == "" {
		return
	}

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
