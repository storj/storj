// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/pb"
)

// semVerRegex is the regular expression used to parse a semantic version.
// https://github.com/Masterminds/semver/blob/master/LICENSE.txt
const (
	semVerRegex string = `v?([0-9]+)\.([0-9]+)\.([0-9]+)`
	quote              = byte('"')
)

var (
	mon = monkit.Package()

	// VerError is the error class for version-related errors.
	VerError = errs.Class("version error")

	// the following fields are set by linker flags. if any of them
	// are set and fail to parse, the program will fail to start
	buildTimestamp  string // unix seconds since epoch
	buildCommitHash string
	buildVersion    string // semantic version format
	buildRelease    string // true/false

	// Build is a struct containing all relevant build information associated with the binary
	Build Info

	versionRegex = regexp.MustCompile("^" + semVerRegex + "$")
)

// Info is the versioning information for a binary
type Info struct {
	// sync/atomic cache
	commitHashCRC uint32

	Timestamp  time.Time `json:"timestamp,omitempty"`
	CommitHash string    `json:"commitHash,omitempty"`
	Version    SemVer    `json:"version"`
	Release    bool      `json:"release,omitempty"`
}

// SemVer represents a semantic version
type SemVer struct {
	Major int64 `json:"major"`
	Minor int64 `json:"minor"`
	Patch int64 `json:"patch"`
}

// AllowedVersions provides the Minimum SemVer per Service
// TODO: I don't think this name is representative of what this struct now holds.
type AllowedVersions struct {
	Satellite   SemVer
	Storagenode SemVer
	Uplink      SemVer
	Gateway     SemVer
	Identity    SemVer

	Processes Processes `json:"processes"`
}

// Processes describes versions for each binary.
// TODO: this name is inconsistent with the versioncontrol server pkg's analogue, `Versions`.
type Processes struct {
	Satellite   Process `json:"satellite"`
	Storagenode Process `json:"storagenode"`
	Uplink      Process `json:"uplink"`
	Gateway     Process `json:"gateway"`
	Identity    Process `json:"identity"`
}

// Process versions for specific binary.
type Process struct {
	Minimum   Version `json:"minimum"`
	Suggested Version `json:"suggested"`
	Rollout   Rollout `json:"rollout"`
}

// Version represents version and download URL for binary.
type Version struct {
	Version string `json:"version"`
	URL     string `json:"url"`
}

// Rollout represents the state of a version rollout.
type Rollout struct {
	Seed   RolloutBytes `json:"seed"`
	Cursor RolloutBytes `json:"cursor"`
}

// RolloutBytes implements json un/marshalling using hex de/encoding.
type RolloutBytes [32]byte

// MarshalJSON hex-encodes RolloutBytes and pre/appends JSON string literal quotes.
func (rb RolloutBytes) MarshalJSON() ([]byte, error) {
	hexBytes := make([]byte, hex.EncodedLen(len(rb)))
	hex.Encode(hexBytes, rb[:])
	encoded := append([]byte{quote}, hexBytes...)
	encoded = append(encoded, quote)
	return encoded, nil
}

// UnmarshalJSON drops the JSON string literal quotes and hex-decodes RolloutBytes .
func (rb *RolloutBytes) UnmarshalJSON(b []byte) error {
	if _, err := hex.Decode(rb[:], b[1:len(b)-1]); err != nil {
		return VerError.Wrap(err)
	}
	return nil
}

// NewSemVer parses a given version and returns an instance of SemVer or
// an error if unable to parse the version.
func NewSemVer(v string) (sv SemVer, err error) {
	m := versionRegex.FindStringSubmatch(v)
	if m == nil {
		return SemVer{}, VerError.New("invalid semantic version for build %s", v)
	}

	// first entry of m is the entire version string
	sv.Major, err = strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return SemVer{}, VerError.Wrap(err)
	}

	sv.Minor, err = strconv.ParseInt(m[2], 10, 64)
	if err != nil {
		return SemVer{}, VerError.Wrap(err)
	}

	sv.Patch, err = strconv.ParseInt(m[3], 10, 64)
	if err != nil {
		return SemVer{}, VerError.Wrap(err)
	}

	return sv, nil
}

// Compare compare two versions, return -1 if compared version is greater, 0 if equal and 1 if less.
func (sem *SemVer) Compare(version SemVer) int {
	result := sem.Major - version.Major
	if result > 0 {
		return 1
	} else if result < 0 {
		return -1
	}
	result = sem.Minor - version.Minor
	if result > 0 {
		return 1
	} else if result < 0 {
		return -1
	}
	result = sem.Patch - version.Patch
	if result > 0 {
		return 1
	} else if result < 0 {
		return -1
	}
	return 0
}

// String converts the SemVer struct to a more easy to handle string
func (sem *SemVer) String() (version string) {
	return fmt.Sprintf("v%d.%d.%d", sem.Major, sem.Minor, sem.Patch)
}

// New creates Version_Info from a json byte array
func New(data []byte) (v Info, err error) {
	err = json.Unmarshal(data, &v)
	return v, VerError.Wrap(err)
}

// Marshal converts the existing Version Info to any json byte array
func (v Info) Marshal() ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, VerError.Wrap(err)
	}
	return data, nil
}

// Proto converts an Info struct to a pb.NodeVersion
// TODO: shouldn't we just use pb.NodeVersion everywhere? gogoproto will let
// us make it match Info.
func (v Info) Proto() (*pb.NodeVersion, error) {
	return &pb.NodeVersion{
		Version:    v.Version.String(),
		CommitHash: v.CommitHash,
		Timestamp:  v.Timestamp,
		Release:    v.Release,
	}, nil
}

func init() {
	if buildVersion == "" && buildTimestamp == "" && buildCommitHash == "" && buildRelease == "" {
		return
	}
	timestamp, err := strconv.ParseInt(buildTimestamp, 10, 64)
	if err != nil {
		panic(VerError.Wrap(err))
	}
	Build = Info{
		Timestamp:  time.Unix(timestamp, 0),
		CommitHash: buildCommitHash,
		Release:    strings.ToLower(buildRelease) == "true",
	}

	sv, err := NewSemVer(buildVersion)
	if err != nil {
		panic(err)
	}

	Build.Version = sv

	if Build.Timestamp.Unix() == 0 || Build.CommitHash == "" {
		Build.Release = false
	}

}
