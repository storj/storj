// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/memory"
	"storj.io/storj/satellite/nodeselection"
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
	NodeCheckInWaitPeriod           time.Duration `help:"the amount of time to wait before accepting a redundant check-in from a node (unmodified info since last check-in)" default:"1h10m" testDefault:"30s"`
	NodeSoftwareUpdateEmailCooldown time.Duration `help:"the amount of time to wait between sending Node Software Update emails" default:"168h"`
	RepairExcludedCountryCodes      []string      `help:"list of country codes to exclude nodes from target repair selection" default:"" testDefault:"FR,BE"`
	SendNodeEmails                  bool          `help:"whether to send emails to nodes" default:"false"`
	NodeTagsIPPortEmails            []string      `help:"comma separated list of node tags for whom to add last ip and port to emails. Currently only for offline emails." default:""`
	MinimumNewNodeIDDifficulty      int           `help:"the minimum node id difficulty required for new nodes. existing nodes remain allowed" devDefault:"0" releaseDefault:"36"`
	AsOfSystemTime                  time.Duration `help:"default AS OF SYSTEM TIME for service" default:"-10s" testDefault:"0"`
}

// AsOfSystemTimeConfig is a configuration struct to enable 'AS OF SYSTEM TIME' for CRDB queries.
type AsOfSystemTimeConfig struct {
	Enabled         bool          `help:"enables the use of the AS OF SYSTEM TIME feature in CRDB" default:"true"`
	DefaultInterval time.Duration `help:"default duration for AS OF SYSTEM TIME" devDefault:"-1ms" releaseDefault:"-10s" testDefault:"-1µs"`
}

// NodeSelectionConfig is a configuration struct to determine the minimum
// values for nodes to select.
type NodeSelectionConfig struct {
	NewNodeFraction   float64       `help:"the fraction of new nodes allowed per request (DEPRECATED: use placement definition instead)" releaseDefault:"0.01" devDefault:"1"`
	MinimumVersion    string        `help:"the minimum node software version for node selection queries" default:""`
	OnlineWindow      time.Duration `help:"the amount of time without seeing a node before its considered offline" default:"4h" testDefault:"5m"`
	DistinctIP        bool          `help:"require distinct IPs when choosing nodes for upload" releaseDefault:"true" devDefault:"false"`
	NetworkPrefixIPv4 int           `help:"the prefix to use in determining 'network' for IPv4 addresses" default:"24" hidden:"true"`
	NetworkPrefixIPv6 int           `help:"the prefix to use in determining 'network' for IPv6 addresses" default:"64" hidden:"true"`
	MinimumDiskSpace  memory.Size   `help:"how much disk space a node at minimum must have to be selected for upload" default:"5.00GB" testDefault:"100.00MB"`

	AsOfSystemTime AsOfSystemTimeConfig

	UploadExcludedCountryCodes []string `help:"list of country codes to exclude from node selection for uploads (DEPRECATED: use placement definition instead)" default:"" testDefault:"FR,BE"`
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

// CreateDefaultPlacement creates a placement (which will be used as default) based on configuration.
// This is used only if no placement is configured, but we need a 0 placement rule.
func (c NodeSelectionConfig) CreateDefaultPlacement() (nodeselection.Placement, error) {
	placement := nodeselection.Placement{
		NodeFilter:       nodeselection.AnyFilter{},
		Selector:         nodeselection.UnvettedSelector(c.NewNodeFraction, nodeselection.AttributeGroupSelector(nodeselection.LastNetAttribute)),
		Invariant:        nodeselection.ClumpingByAttribute(nodeselection.LastNetAttribute, 1),
		DownloadSelector: nodeselection.DefaultDownloadSelector,
	}
	if len(c.UploadExcludedCountryCodes) > 0 {
		countryFilter, err := nodeselection.NewCountryFilterFromString(c.UploadExcludedCountryCodes)
		if err != nil {
			return nodeselection.Placement{}, err
		}
		placement.UploadFilter = nodeselection.NewExcludeFilter(countryFilter)
	}

	return placement, nil
}
