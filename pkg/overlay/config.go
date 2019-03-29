// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"strings"

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
	UptimeRatio       float64 `help:"a node's ratio of being up/online vs. down/offline" default:"0"`
	UptimeCount       int64   `help:"the number of times a node's uptime has been checked" default:"0"`
	AuditSuccessRatio float64 `help:"a node's ratio of successful audits" default:"0"`
	AuditCount        int64   `help:"the number of times a node has been audited" default:"0"`

	NewNodeAuditThreshold int64   `help:"the number of audits a node must have to not be considered a New Node" default:"0"`
	NewNodePercentage     float64 `help:"the percentage of new nodes allowed per request" default:"0.05"` // TODO: fix, this is not percentage, it's ratio
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
