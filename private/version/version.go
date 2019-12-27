// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package version

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

const quote = byte('"')

var (
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

// SemVer represents a semantic version.
// TODO: replace with semver.Version
type SemVer struct {
	semver.Version
}

// OldSemVer represents a semantic version.
// NB: this will be deprecated in favor of `SemVer`; these structs marshal to JSON differently.
type OldSemVer struct {
	Major int64 `json:"major"`
	Minor int64 `json:"minor"`
	Patch int64 `json:"patch"`
}

// AllowedVersions provides the Minimum SemVer per Service.
// TODO: I don't think this name is representative of what this struct now holds.
type AllowedVersions struct {
	Satellite   OldSemVer
	Storagenode OldSemVer
	Uplink      OldSemVer
	Gateway     OldSemVer
	Identity    OldSemVer

	Processes Processes `json:"processes"`
}

// Processes describes versions for each binary.
// TODO: this name is inconsistent with the versioncontrol server pkg's analogue, `Versions`.
type Processes struct {
	Satellite          Process `json:"satellite"`
	Storagenode        Process `json:"storagenode"`
	StoragenodeUpdater Process `json:"storagenode-updater"`
	Uplink             Process `json:"uplink"`
	Gateway            Process `json:"gateway"`
	Identity           Process `json:"identity"`
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
	zeroRolloutBytes := RolloutBytes{}
	if bytes.Equal(rb[:], zeroRolloutBytes[:]) {
		return []byte{quote, quote}, nil
	}

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
func NewSemVer(v string) (SemVer, error) {
	ver, err := semver.ParseTolerant(v)
	if err != nil {
		return SemVer{}, VerError.Wrap(err)
	}

	return SemVer{
		Version: ver,
	}, nil
}

// NewOldSemVer parses a given version and returns an instance of OldSemVer or
// an error if unable to parse the version.
func NewOldSemVer(v string) (OldSemVer, error) {
	ver, err := NewSemVer(v)
	if err != nil {
		return OldSemVer{}, err
	}

	return OldSemVer{
		Major: int64(ver.Major),
		Minor: int64(ver.Minor),
		Patch: int64(ver.Patch),
	}, nil
}

// Compare compare two versions, return -1 if compared version is greater, 0 if equal and 1 if less.
func (sem *SemVer) Compare(version SemVer) int {
	return sem.Version.Compare(version.Version)
}

// String converts the SemVer struct to a more easy to handle string
func (sem *SemVer) String() (version string) {
	return fmt.Sprintf("v%d.%d.%d", sem.Major, sem.Minor, sem.Patch)
}

// IsZero checks if the semantic version is its zero value.
func (sem SemVer) IsZero() bool {
	return reflect.ValueOf(sem).IsZero()
}

func (old OldSemVer) String() string {
	return fmt.Sprintf("v%d.%d.%d", old.Major, old.Minor, old.Patch)
}

// SemVer converts a version struct into a semantic version struct.
func (ver *Version) SemVer() (SemVer, error) {
	return NewSemVer(ver.Version)
}

// New creates Version_Info from a json byte array
func New(data []byte) (v Info, err error) {
	err = json.Unmarshal(data, &v)
	return v, VerError.Wrap(err)
}

// IsZero checks if the version struct is its zero value.
func (info Info) IsZero() bool {
	return reflect.ValueOf(info).IsZero()
}

// Marshal converts the existing Version Info to any json byte array
func (info Info) Marshal() ([]byte, error) {
	data, err := json.Marshal(info)
	if err != nil {
		return nil, VerError.Wrap(err)
	}
	return data, nil
}

// Proto converts an Info struct to a pb.NodeVersion
// TODO: shouldn't we just use pb.NodeVersion everywhere? gogoproto will let
// us make it match Info.
func (info Info) Proto() (*pb.NodeVersion, error) {
	return &pb.NodeVersion{
		Version:    info.Version.String(),
		CommitHash: info.CommitHash,
		Timestamp:  info.Timestamp,
		Release:    info.Release,
	}, nil
}

// PercentageToCursor calculates the cursor value for the given percentage of nodes which should update.
func PercentageToCursor(pct int) RolloutBytes {
	// NB: convert the max value to a number, multiply by the percentage, convert back.
	var maxInt, maskInt big.Int
	var maxBytes RolloutBytes
	for i := 0; i < len(maxBytes); i++ {
		maxBytes[i] = 255
	}
	maxInt.SetBytes(maxBytes[:])
	maskInt.Div(maskInt.Mul(&maxInt, big.NewInt(int64(pct))), big.NewInt(100))

	var cursor RolloutBytes
	copy(cursor[:], maskInt.Bytes())

	return cursor
}

// ShouldUpdate checks if for the the given rollout state, a user with the given nodeID should update.
func ShouldUpdate(rollout Rollout, nodeID storj.NodeID) bool {
	hash := hmac.New(sha256.New, rollout.Seed[:])
	_, err := hash.Write(nodeID[:])
	if err != nil {
		panic(err)
	}
	return bytes.Compare(hash.Sum(nil), rollout.Cursor[:]) <= 0
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
