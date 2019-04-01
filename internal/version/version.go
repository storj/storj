// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	monkit "gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	mon = monkit.Package()
	// Timestamp is the UTC timestamp of the compilation time
	Timestamp int64
	// CommitHash is the git hash of the code being compiled
	CommitHash string
	// Version is the semantic version set at compilation
	// if not a valid semantic version Release should be false
	Version = "v0.1.0"
	// Release indicates whether the binary compiled is a release candidate
	Release bool
	// Build is a struct containing all relevant build information associated with the binary
	Build Info
)

// Info is the versioning information for a binary
type Info struct {
	Timestamp  int64  `json:"timestamp,omitempty"`
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

// AllowedVersions provides a list of SemVer per Service
type AllowedVersions struct {
	Bootstrap   []SemVer
	Satellite   []SemVer
	Storagenode []SemVer
	Uplink      []SemVer
	Gateway     []SemVer
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
func New(data []byte) (v Info, err error) {
	err = json.Unmarshal(data, &v)
	return v, err
}

// Marshal converts the existing Version Info to any json byte array
func (v Info) Marshal() (data []byte, err error) {
	data, err = json.Marshal(v)
	return
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
func containsVersion(all []SemVer, x SemVer) bool {
	for _, n := range all {
		if x == n {
			return true
		}
	}
	return false
}

// StrToSemVerList converts a list of versions to a list of SemVer
func StrToSemVerList(serviceVersions []string) (versions []SemVer, err error) {

	versionRegex := regexp.MustCompile("^" + SemVerRegex + "$")

	for _, subversion := range serviceVersions {
		sVer, err := NewSemVer(versionRegex, subversion)
		if err != nil {
			return nil, err
		}
		versions = append(versions, *sVer)
	}
	return versions, err
}

func init() {
	if Version == "" {
		return
	}

	Build = Info{
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

	if Build.Timestamp == 0 || Build.CommitHash == "" {
		Build.Release = false
	}
}
