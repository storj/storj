// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"strings"
	"time"

	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/storj"
)

var (
	mon = monkit.Package()
	// Error represents an overlay error
	Error = errs.Class("overlay error")
)

// Config is a configuration struct for everything you need to start the
// Overlay cache responsibility.
type Config struct {
	Node NodeSelectionConfig
}

// LookupConfig is a configuration struct for querying the overlay cache with one or more node IDs
type LookupConfig struct {
	NodeIDsString string `help:"one or more string-encoded node IDs, delimited by Delimiter"`
	Delimiter     string `help:"delimiter used for parsing node IDs" default:","`
}

// NodeSelectionConfig is a configuration struct to determine the minimum
// values for nodes to select
type NodeSelectionConfig struct {
	UptimeCount       int64         `help:"the number of times a node's uptime has been checked to not be considered a New Node" releaseDefault:"500" devDefault:"0"`
	AuditCount        int64         `help:"the number of times a node has been audited to not be considered a New Node" releaseDefault:"500" devDefault:"0"`
	NewNodePercentage float64       `help:"the percentage of new nodes allowed per request" default:"0.05"` // TODO: fix, this is not percentage, it's ratio
	MinimumVersion    string        `help:"the minimum node software version for node selection queries" default:""`
	OnlineWindow      time.Duration `help:"the amount of time without seeing a node before its considered offline" default:"1h"`
	DistinctIP        bool          `help:"require distinct IPs when choosing nodes for upload" releaseDefault:"true" devDefault:"false"`

	ReputationAuditRepairWeight  float64 `help:"weight to apply to audit reputation for total repair reputation calculation" default:"1.0"`
	ReputationAuditUplinkWeight  float64 `help:"weight to apply to audit reputation for total uplink reputation calculation" default:"1.0"`
	ReputationAuditAlpha0        float64 `help:"the initial shape 'alpha' used to calculate audit SNs reputation" default:"1.0"`
	ReputationAuditBeta0         float64 `help:"the initial shape 'beta' value used to calculate audit SNs reputation" default:"0.0"`
	ReputationAuditLambda        float64 `help:"the forgetting factor used to calculate the audit SNs reputation" default:"1.0"`
	ReputationAuditOmega         float64 `help:"the normalization weight used to calculate the audit SNs reputation" default:"1.0"`
	ReputationUptimeRepairWeight float64 `help:"weight to apply to uptime reputation for total repair reputation calculation" default:"1.0"`
	ReputationUptimeUplinkWeight float64 `help:"weight to apply to uptime reputation for total uplink reputation calculation" default:"1.0"`
	ReputationUptimeAlpha0       float64 `help:"the initial shape 'alpha' used to calculate uptime SNs reputation" default:"1.0"`
	ReputationUptimeBeta0        float64 `help:"the initial shape 'beta' value used to calculate uptime SNs reputation" default:"0.0"`
	ReputationUptimeLambda       float64 `help:"the forgetting factor used to calculate the uptime SNs reputation" default:"1.0"`
	ReputationUptimeOmega        float64 `help:"the normalization weight used to calculate the uptime SNs reputation" default:"1.0"`
}

// ParseIDs converts the base58check encoded node ID strings from the config into node IDs
func (c LookupConfig) ParseIDs() (ids storj.NodeIDList, err error) {
	var idErrs []error
	idStrs := strings.Split(c.NodeIDsString, c.Delimiter)
	for _, s := range idStrs {
		id, err := storj.NodeIDFromString(s)
		if err != nil {
			idErrs = append(idErrs, err)
			continue
		}
		ids = append(ids, id)
	}
	if err := errs.Combine(idErrs...); err != nil {
		return nil, err
	}
	return ids, nil
}
