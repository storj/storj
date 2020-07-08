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
	Error = errs.Class("overlay error")
)

// Config is a configuration for overlay service.
type Config struct {
	Node                 NodeSelectionConfig
	NodeSelectionCache   CacheConfig
	UpdateStatsBatchSize int `help:"number of update requests to process per transaction" default:"100"`
	AuditHistory         AuditHistoryConfig
}

// NodeSelectionConfig is a configuration struct to determine the minimum
// values for nodes to select.
type NodeSelectionConfig struct {
	UptimeCount      int64         `help:"the number of times a node's uptime has been checked to not be considered a New Node" releaseDefault:"100" devDefault:"0"`
	AuditCount       int64         `help:"the number of times a node has been audited to not be considered a New Node" releaseDefault:"100" devDefault:"0"`
	NewNodeFraction  float64       `help:"the fraction of new nodes allowed per request" releaseDefault:"0.05" devDefault:"1"`
	MinimumVersion   string        `help:"the minimum node software version for node selection queries" default:""`
	OnlineWindow     time.Duration `help:"the amount of time without seeing a node before its considered offline" default:"4h"`
	DistinctIP       bool          `help:"require distinct IPs when choosing nodes for upload" releaseDefault:"true" devDefault:"false"`
	MinimumDiskSpace memory.Size   `help:"how much disk space a node at minimum must have to be selected for upload" default:"500.00MB"`

	AuditReputationRepairWeight float64       `help:"weight to apply to audit reputation for total repair reputation calculation" default:"1.0"`
	AuditReputationUplinkWeight float64       `help:"weight to apply to audit reputation for total uplink reputation calculation" default:"1.0"`
	AuditReputationLambda       float64       `help:"the forgetting factor used to calculate the audit SNs reputation" default:"0.95"`
	AuditReputationWeight       float64       `help:"the normalization weight used to calculate the audit SNs reputation" default:"1.0"`
	AuditReputationDQ           float64       `help:"the reputation cut-off for disqualifying SNs based on audit history" default:"0.6"`
	SuspensionGracePeriod       time.Duration `help:"the time period that must pass before suspended nodes will be disqualified" releaseDefault:"168h" devDefault:"1h"`
	SuspensionDQEnabled         bool          `help:"whether nodes will be disqualified if they have been suspended for longer than the suspended grace period" releaseDefault:"false" devDefault:"true"`
}

// AuditHistoryConfig is a configuration struct defining time periods and thresholds for penalizing nodes for being offline.
// It is used for downtime suspension and disqualification.
type AuditHistoryConfig struct {
	WindowSize       time.Duration `help:"The length of time spanning a single audit window" releaseDefault:"12h" devDefault:"5m"`
	TrackingPeriod   time.Duration `help:"The length of time to track audit windows for node suspension and disqualification" releaseDefault:"720h" devDefault:"1h"`
	GracePeriod      time.Duration `help:"The length of time to give suspended SNOs to diagnose and fix issues causing downtime. Afterwards, they will have one tracking period to reach the minimum online score before disqualification" releaseDefault:"168h" devDefault:"1h"`
	OfflineThreshold float64       `help:"The point below which a node is punished for offline audits. Determined by calculating the ratio of online/total audits within each window and finding the average across windows within the tracking period." default:"0.6"`
}
