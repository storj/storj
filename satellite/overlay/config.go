// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/memory"
)

var (
	mon = monkit.Package()
	// Error represents an overlay error.
	Error = errs.Class("overlay")
)

// Config is a configuration for overlay service.
type Config struct {
	Node                            NodeSelectionConfig
	NodeSelectionCache              UploadSelectionCacheConfig
	GeoIP                           GeoIPConfig
	UpdateStatsBatchSize            int           `help:"number of update requests to process per transaction" default:"100"`
	NodeCheckInWaitPeriod           time.Duration `help:"the amount of time to wait before accepting a redundant check-in from a node (unmodified info since last check-in)" default:"2h" testDefault:"30s"`
	NodeSoftwareUpdateEmailCooldown time.Duration `help:"the amount of time to wait between sending Node Software Update emails" default:"168h"`
	RepairExcludedCountryCodes      []string      `help:"list of country codes to exclude nodes from target repair selection" default:"" testDefault:"FR,BE"`
	SendNodeEmails                  bool          `help:"whether to send emails to nodes" default:"false"`
	MinimumNewNodeIDDifficulty      int           `help:"the minimum node id difficulty required for new nodes. existing nodes remain allowed" devDefault:"0" releaseDefault:"36"`
}

// AsOfSystemTimeConfig is a configuration struct to enable 'AS OF SYSTEM TIME' for CRDB queries.
type AsOfSystemTimeConfig struct {
	Enabled         bool          `help:"enables the use of the AS OF SYSTEM TIME feature in CRDB" default:"true"`
	DefaultInterval time.Duration `help:"default duration for AS OF SYSTEM TIME" devDefault:"-1ms" releaseDefault:"-10s" testDefault:"-1µs"`
}

// NodeSelectionConfig is a configuration struct to determine the minimum
// values for nodes to select.
type NodeSelectionConfig struct {
	NewNodeFraction   float64       `help:"the fraction of new nodes allowed per request" releaseDefault:"0.05" devDefault:"1"`
	MinimumVersion    string        `help:"the minimum node software version for node selection queries" default:""`
	OnlineWindow      time.Duration `help:"the amount of time without seeing a node before its considered offline" default:"4h" testDefault:"1m"`
	DistinctIP        bool          `help:"require distinct IPs when choosing nodes for upload" releaseDefault:"true" devDefault:"false"`
	NetworkPrefixIPv4 int           `help:"the prefix to use in determining 'network' for IPv4 addresses" default:"24" hidden:"true"`
	NetworkPrefixIPv6 int           `help:"the prefix to use in determining 'network' for IPv6 addresses" default:"64" hidden:"true"`
	MinimumDiskSpace  memory.Size   `help:"how much disk space a node at minimum must have to be selected for upload" default:"500.00MB" testDefault:"100.00MB"`

	AsOfSystemTime AsOfSystemTimeConfig

	UploadExcludedCountryCodes []string `help:"list of country codes to exclude from node selection for uploads" default:"" testDefault:"FR,BE"`
}

// GeoIPConfig is a configuration struct that helps configure the GeoIP lookup features on the satellite.
type GeoIPConfig struct {
	DB            string   `help:"the location of the maxmind database containing geoip country information"`
	MockCountries []string `help:"a mock list of countries the satellite will attribute to nodes (useful for testing)"`
}

func (aost *AsOfSystemTimeConfig) isValid() error {
	if aost.Enabled {
		if aost.DefaultInterval >= 0 {
			return errs.New("AS OF SYSTEM TIME interval must be a negative number")
		}
		if aost.DefaultInterval > -time.Microsecond {
			return errs.New("AS OF SYSTEM TIME interval cannot be in nanoseconds")
		}
	}

	return nil
}

// Interval returns the configured interval respecting Enabled property.
func (aost *AsOfSystemTimeConfig) Interval() time.Duration {
	if !aost.Enabled {
		return 0
	}
	return aost.DefaultInterval
}
