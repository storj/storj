// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"storj.io/common/memory"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/uplink/private/eestream"
)

const (
	// BoltPointerBucket is the string representing the bucket used for `PointerEntries` in BoltDB.
	BoltPointerBucket = "pointers"
)

// RSConfig is a configuration struct that keeps details about default
// redundancy strategy information.
//
// Can be used as a flag.
type RSConfig struct {
	ErasureShareSize memory.Size
	Min              int
	Repair           int
	Success          int
	Total            int
}

// Type implements pflag.Value.
func (RSConfig) Type() string { return "metainfo.RSConfig" }

// String is required for pflag.Value.
func (rs *RSConfig) String() string {
	return fmt.Sprintf("%d/%d/%d/%d-%s",
		rs.Min,
		rs.Repair,
		rs.Success,
		rs.Total,
		rs.ErasureShareSize.String())
}

// Override creates a new RSConfig instance, all non-zero parameters of o will be used to override current values.
func (rs *RSConfig) Override(o nodeselection.ECParameters) *RSConfig {
	ro := &RSConfig{
		ErasureShareSize: rs.ErasureShareSize,
		Min:              rs.Min,
		Repair:           rs.Repair,
		Success:          rs.Success,
		Total:            rs.Total,
	}
	if o.Minimum > 0 {
		ro.Min = o.Minimum
	}

	if o.Success != nil {
		if v := o.Success(o.Minimum); v > 0 {
			ro.Success = v
		}
	}

	if o.Repair != nil {
		if v := o.Repair(o.Minimum); v > 0 {
			ro.Repair = v
		}
	}

	if o.Total > 0 {
		ro.Total = o.Total
		// for legacy configuration that does not define repair.
		// we need to adjust to avoid validation error
		if ro.Repair > ro.Total {
			ro.Repair = ro.Total
		}
	}

	return ro
}

// Set sets the value from a string in the format satellite/satellitedb/overlaycache.gok/m/o/n-size (min/repair/optimal/total-erasuresharesize).
func (rs *RSConfig) Set(s string) error {
	// Split on dash. Expect two items. First item is RS numbers. Second item is memory.Size.
	info := strings.Split(s, "-")
	if len(info) != 2 {
		return Error.New("Invalid default RS config (expect format k/m/o/n-ShareSize, got %s)", s)
	}
	rsNumbersString := info[0]
	shareSizeString := info[1]

	// Attempt to parse "-size" part of config.
	shareSizeInt, err := memory.ParseString(shareSizeString)
	if err != nil {
		return Error.New("Invalid share size in RS config: '%s', %w", shareSizeString, err)
	}
	shareSize := memory.Size(shareSizeInt)

	// Split on forward slash. Expect exactly four positive non-decreasing integers.
	rsNumbers := strings.Split(rsNumbersString, "/")
	if len(rsNumbers) != 4 {
		return Error.New("Invalid default RS numbers (wrong size, expect 4): %s", rsNumbersString)
	}

	minValue := 1
	values := []int{}
	for _, nextValueString := range rsNumbers {
		nextValue, err := strconv.Atoi(nextValueString)
		if err != nil {
			return Error.New("Invalid default RS numbers (should all be valid integers): %s, %w", rsNumbersString, err)
		}
		if nextValue < minValue {
			return Error.New("Invalid default RS numbers (should be non-decreasing): %s", rsNumbersString)
		}
		values = append(values, nextValue)
		minValue = nextValue
	}

	rs.ErasureShareSize = shareSize
	rs.Min = values[0]
	rs.Repair = values[1]
	rs.Success = values[2]
	rs.Total = values[3]

	return nil
}

// RedundancyStrategy creates eestream.RedundancyStrategy from config values.
func (rs *RSConfig) RedundancyStrategy() (eestream.RedundancyStrategy, error) {
	fec, err := eestream.NewFEC(rs.Min, rs.Total)
	if err != nil {
		return eestream.RedundancyStrategy{}, err
	}
	erasureScheme := eestream.NewRSScheme(fec, rs.ErasureShareSize.Int())
	return eestream.NewRedundancyStrategy(erasureScheme, rs.Repair, rs.Success)
}

// RateLimiterConfig is a configuration struct for endpoint rate limiting.
type RateLimiterConfig struct {
	Enabled         bool          `help:"whether rate limiting is enabled." releaseDefault:"true" devDefault:"true"`
	Rate            float64       `help:"request rate per project per second." releaseDefault:"100" devDefault:"100" testDefault:"1000"`
	CacheCapacity   int           `help:"number of projects to cache." releaseDefault:"10000" devDefault:"10" testDefault:"100"`
	CacheExpiration time.Duration `help:"how long to cache the projects limiter." releaseDefault:"10m" devDefault:"10s"`
}

// UploadLimiterConfig is a configuration struct for endpoint upload limiting.
type UploadLimiterConfig struct {
	Enabled           bool          `help:"whether rate limiting is enabled." releaseDefault:"true" devDefault:"true"`
	SingleObjectLimit time.Duration `help:"how often we can upload to the single object (the same location) per API instance" default:"1s" devDefault:"1ms"`

	CacheCapacity int `help:"DEPRECATED. number of object locations to cache." releaseDefault:"10000" devDefault:"10" testDefault:"100"`

	BurstLimit   int `help:"the number of requests to allow bursts beyond the rate limit" default:"3"`
	HashCount    int `help:"the number of hash indexes to make into the rate limit map" default:"3"`
	SizeExponent int `help:"two to this power is the amount of rate limits to store in ram. higher has less collisions." releaseDefault:"21" devDefault:"17" testDefault:"16"`
}

// DownloadLimiterConfig is a configuration struct for endpoint download limiting.
type DownloadLimiterConfig struct {
	Enabled           bool          `help:"whether rate limiting is enabled." releaseDefault:"true" devDefault:"true"`
	SingleObjectLimit time.Duration `help:"how often we can upload to the single object (the same location) per API instance" default:"1ms"`
	BurstLimit        int           `help:"the number of requests to allow bursts beyond the rate limit" default:"3"`
	HashCount         int           `help:"the number of hash indexes to make into the rate limit map" default:"3"`
	SizeExponent      int           `help:"two to this power is the amount of rate limits to store in ram. higher has less collisions." releaseDefault:"21" devDefault:"17" testDefault:"16"`
}

// ProjectLimitConfig is a configuration struct for default project limits.
type ProjectLimitConfig struct {
	MaxBuckets int `help:"max bucket count for a project." default:"100" testDefault:"10"`
}

// UserInfoValidationConfig is a configuration struct for user info validation.
type UserInfoValidationConfig struct {
	Enabled         bool          `help:"whether validation is enabled for user account info" default:"false"`
	CacheExpiration time.Duration `help:"user info cache expiration" default:"5m"`
	CacheCapacity   int           `help:"user info cache capacity" default:"10000"`
}

// APIKeyTailsConfig is a configuration struct for API key tails processing.
type APIKeyTailsConfig struct {
	CombinerQueueEnabled bool          `help:"whether combiner queue is enabled for processing API key tails" default:"false"`
	QueueSize            int           `help:"size of API key tails combiner queue" default:"100"`
	CacheExpiration      time.Duration `help:"API key tails cache expiration" default:"5m"`
	CacheCapacity        int           `help:"API key tails cache capacity" default:"10000"`
}

// Config is a configuration struct that is everything you need to start a metainfo.
type Config struct {
	DatabaseURL          string      `help:"the database connection string to use" default:"postgres://"`
	MinRemoteSegmentSize memory.Size `default:"1240" testDefault:"0" help:"minimum remote segment size"` // TODO: fix tests to work with 1024
	MaxInlineSegmentSize memory.Size `default:"4KiB" help:"maximum inline segment size"`
	// we have such default value because max value for ObjectKey is 1024(1 Kib) but EncryptedObjectKey
	// has encryption overhead 16 bytes. So overall size is 1024 + 16 * 16.
	MaxEncryptedObjectKeyLength  int                   `default:"4000" help:"maximum encrypted object key length"`
	MaxSegmentSize               memory.Size           `default:"64MiB" help:"maximum segment size"`
	MaxMetadataSize              memory.Size           `default:"2KiB" help:"maximum segment metadata size"`
	MaxCommitInterval            time.Duration         `default:"48h" testDefault:"1h" help:"maximum time allowed to pass between creating and committing a segment"`
	MinPartSize                  memory.Size           `default:"5MiB" testDefault:"0" help:"minimum allowed part size (last part has no minimum size limit)"`
	MaxNumberOfParts             int                   `default:"10000" help:"maximum number of parts object can contain"`
	Overlay                      bool                  `default:"true" help:"toggle flag if overlay is enabled"`
	RS                           RSConfig              `releaseDefault:"29/35/80/110-256B" devDefault:"4/6/8/10-256B" help:"redundancy scheme configuration in the format k/m/o/n-sharesize"`
	RateLimiter                  RateLimiterConfig     `help:"rate limiter configuration"`
	UploadLimiter                UploadLimiterConfig   `help:"object upload limiter configuration"`
	DownloadLimiter              DownloadLimiterConfig `help:"object download limiter configuration"`
	ProjectLimits                ProjectLimitConfig    `help:"project limit configuration"`
	SuccessTrackerKind           string                `default:"percent" help:"success tracker kind, bitshift or percent"`
	SuccessTrackerTickDuration   time.Duration         `default:"10m" help:"how often to bump the generation in the node success tracker"`
	FailureTrackerTickDuration   time.Duration         `default:"5s" help:"how often to bump the generation in the node failure tracker"`
	SuccessTrackerTrustedUplinks []string              `help:"list of trusted uplinks for success tracker, deprecated. please use success-tracker-uplinks for uplinks that should get their own success tracker profiles and trusted-uplinks for uplinks that are trusted individually."`
	SuccessTrackerUplinks        []string              `help:"list of uplinks for success tracker"`
	SuccessTrackerMonitorEnabled bool                  `help:"enable monkit monitoring of success tracker" default:"false"`
	SuccessTrackerMonitorFilter  string                `help:"filter for nodes that should be monitored by success tracker monitor" default:"none()"`
	FailureTrackerChanceToSkip   float64               `help:"the chance to skip a failure tracker generation bump" default:".6"`
	TrustedUplinks               []string              `help:"list of trusted uplinks"`
	AlwaysUpdateGlobalTracker    bool                  `help:"if true, always update the global tracker with info, even if the uplink is registered" default:"false"`

	// TODO remove this flag when server-side copy implementation will be finished
	ServerSideCopy         bool `help:"enable code for server-side copy, deprecated. please leave this to true." default:"true"`
	ServerSideCopyDisabled bool `help:"disable already enabled server-side copy. this is because once server side copy is enabled, delete code should stay changed, even if you want to disable server side copy" default:"false"`

	NodeAliasCacheFullRefresh bool `help:"node alias cache does a full refresh when a value is missing" default:"false"`

	UseBucketLevelObjectVersioning bool `help:"enable the use of bucket level object versioning" default:"true"`

	UseListObjectsForListing bool `help:"switch to new ListObjects implementation" default:"false" devDefault:"true" testDefault:"true"`

	ListObjects ListObjectsFlags `help:"tuning parameters for list objects"`

	ObjectLockEnabled bool `help:"enable the use of bucket-level Object Lock" default:"true"`

	UserInfoValidation UserInfoValidationConfig `help:"Config for user info validation"`

	SelfServePlacementSelectEnabled bool `help:"whether self-serve placement selection feature is enabled. Provided by console config." default:"false" hidden:"true"`

	SendEdgeUrlOverrides bool `help:"send edge URL overrides through the GetProjectInfo endpoint" default:"false"`

	DeleteObjectsEnabled bool `help:"enable the use of the DeleteObjects endpoint" default:"false"`

	BucketTaggingEnabled bool `help:"enable the use of the bucket tagging endpoints" default:"false"`

	APIKeyTailsConfig APIKeyTailsConfig `help:"Config for API key tails processing"`

	CopyMoveSegmentLimit int64 `help:"the maximum number of segments that can be copied or moved in a single operation" default:"10000"`

	// TODO remove when we benchmarking are done and decision is made.
	TestListingQuery                bool      `default:"false" help:"test the new query for non-recursive listing"`
	TestOptimizedInlineObjectUpload bool      `default:"false" help:"enables optimization for uploading objects with single inline segment"`
	TestingSpannerProjects          UUIDsFlag `default:"" help:"list of project IDs for which Spanner metabase DB is enabled" hidden:"true"`
	TestingMigrationMode            bool      `default:"false" help:"sets metainfo API into migration mode, only read actions are allowed" hidden:"true"`
	TestingTimestampVersioning      bool      `default:"false" help:"use timestamps for assigning version numbers" hidden:"true"`
	TestingTwoRoundtripCommit       bool      `default:"false" help:"Use a new two roundtrip commit object." testDefault:"true" hidden:"true"`

	TestingProjectsWithCommitDelay   UUIDsFlag     `default:"" help:"list of project IDs for which commit delay is enabled" hidden:"true"`
	TestingMaxCommitDelay            time.Duration `default:"20ms" help:"max commit delay that will be used when commit delay is enabled for project" hidden:"true"`
	TestingCommitSegmentUseMutations bool          `default:"false" help:"enable using Spanner mutations while committing segment" hidden:"true"`
	TestingDeleteBucketBatchSize     int           `default:"15" help:"how many objects to delete in a single batch during a bucket deletion"`

	TestingAlternativeBeginObject         bool      `default:"true" help:"enable alternative (negative version) begin object implementation globally" hidden:"true"`
	TestingAlternativeBeginObjectProjects UUIDsFlag `default:"" help:"list of project IDs for which will use alternative (negative version) begin object implementation" hidden:"true"`

	TestingNoPendingObjectUpload         bool      `default:"false" help:"enable alternative upload flow where pending object is not created" hidden:"true"`
	TestingNoPendingObjectUploadProjects UUIDsFlag `default:"" help:"list of project IDs for which will use alternative upload flow where pending object is not created" hidden:"true"`

	// TODO we need to split this into separate config with other metabase related flags
	MetabaseCompression string `help:"Compression type to be used in spanner client for gRPC calls, disabled by default (gzip)" default:"" devDefault:"gzip"`
}

// Metabase constructs Metabase configuration based on Metainfo configuration with specific application name.
func (c Config) Metabase(applicationName string) metabase.Config {
	return metabase.Config{
		ApplicationName:            applicationName,
		MinPartSize:                c.MinPartSize,
		MaxNumberOfParts:           c.MaxNumberOfParts,
		ServerSideCopy:             c.ServerSideCopy,
		NodeAliasCacheFullRefresh:  c.NodeAliasCacheFullRefresh,
		TestingSpannerProjects:     c.TestingSpannerProjects,
		TestingTimestampVersioning: c.TestingTimestampVersioning,
		TestingTwoRoundtripCommit:  c.TestingTwoRoundtripCommit,
		Compression:                c.MetabaseCompression,
	}
}

// UUIDsFlag is a configuration struct that keeps info about project IDs
//
// Can be used as a flag.
type UUIDsFlag map[uuid.UUID]struct{}

// Type is required for pflag.Value.
func (m UUIDsFlag) Type() string {
	return "metainfo.UUIDsFlag"
}

// Set is required for pflag.Value.
func (m *UUIDsFlag) Set(s string) error {
	if s == "" {
		*m = map[uuid.UUID]struct{}{}
		return nil
	}

	uuids := strings.Split(s, ",")
	*m = make(map[uuid.UUID]struct{}, len(uuids))
	for _, uuidStr := range uuids {
		id, err := uuid.FromString(uuidStr)
		if err != nil {
			return err
		}

		(*m)[id] = struct{}{}
	}
	return nil
}

// String is required for pflag.Value.
func (m UUIDsFlag) String() string {
	var b strings.Builder
	i := 0
	for id := range m {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(id.String())
		i++
	}
	return b.String()
}

// MigrationModeFlagExtension defines custom debug endpoint for metainfo migration mode flag.
type MigrationModeFlagExtension struct {
	migrationMode atomic.Bool
}

// NewMigrationModeFlagExtension creates a new instance of MigrationModeFlagExtension.
func NewMigrationModeFlagExtension(config Config) *MigrationModeFlagExtension {
	m := &MigrationModeFlagExtension{}
	m.migrationMode.Store(config.TestingMigrationMode)
	return m
}

// Description is a display name for the UI.
func (m *MigrationModeFlagExtension) Description() string {
	return "give ability to get or set state of metainfo TestingMigrationMode flag"
}

// Path is the unique HTTP path fragment.
func (m *MigrationModeFlagExtension) Path() string {
	return "/metainfo/flags/migration-mode"
}

// Handler is the HTTP handler for the path.
func (m *MigrationModeFlagExtension) Handler(w http.ResponseWriter, r *http.Request) {

	switch r.Method {
	case http.MethodGet:
		_, err := w.Write([]byte(strconv.FormatBool(m.migrationMode.Load())))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, "internal error: %v", err)
		}
	case http.MethodPut:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = fmt.Fprintf(w, "internal error: %v", err)
		}
		value, err := strconv.ParseBool(string(body))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = fmt.Fprintf(w, "internal error: %v", err)
		}
		m.migrationMode.Store(value)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = fmt.Fprintf(w, "Only GET or PUT are supported.")
	}
}

// Enabled returns true if migration mode is enabled.
func (m *MigrationModeFlagExtension) Enabled() bool {
	return m.migrationMode.Load()
}

func (c *Config) isNoPendingObjectUploadEnabled(projectID uuid.UUID) bool {
	if c.TestingNoPendingObjectUpload {
		return true
	}
	if len(c.TestingNoPendingObjectUploadProjects) == 0 {
		return false
	}
	_, exists := c.TestingNoPendingObjectUploadProjects[projectID]
	return exists
}
