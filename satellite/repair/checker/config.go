// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

// Config contains configurable values for checker.
type Config struct {
	Interval time.Duration `help:"how frequently checker should check for bad segments" releaseDefault:"30s" devDefault:"0h0m10s" testDefault:"$TESTINTERVAL"`

	ReliabilityCacheStaleness time.Duration            `help:"how stale reliable node cache can be" releaseDefault:"5m" devDefault:"5m" testDefault:"1m"`
	RepairOverrides           RepairOverrides          `help:"[DEPRECATED] comma-separated override values for repair threshold in the format k-threshold" default:"" deprecated:"true" hidden:"true"`
	RepairThresholdOverrides  RepairThresholdOverrides `help:"comma-separated override values for repair threshold in the format k-threshold" default:""`
	RepairTargetOverrides     RepairTargetOverrides    `help:"comma-separated override values for repair success target in the format k-target" default:""`
	// Node failure rate is an estimation based on a 6 hour checker run interval (4 checker iterations per day), a network of about 9200 nodes, and about 2 nodes churning per day.
	// This results in `2/9200/4 = 0.00005435` being the probability of any single node going down in the interval of one checker iteration.
	NodeFailureRate            float64       `help:"the probability of a single node going down within the next checker iteration" default:"0.00005435" `
	RepairQueueInsertBatchSize int           `help:"Number of damaged segments to buffer in-memory before flushing to the repair queue" default:"100" `
	RepairExcludedCountryCodes []string      `help:"list of country codes to treat node from this country as offline " default:"" hidden:"true"`
	DoDeclumping               bool          `help:"Treat pieces on the same network as in need of repair" default:"true"`
	DoPlacementCheck           bool          `help:"Treat pieces out of segment placement as in need of repair" default:"true"`
	HealthScore                string        `help:"Health score to use for segment health calculation. Options: 'probability', 'normalized'. 'probability' uses the original SegmentHealth logic with node count estimation, while 'normalized' uses a normalized health calculation (healthy -k)." default:"probability" enum:"probability,normalized"`
	OnlineWindow               time.Duration `help:"the amount of time without seeing a node before its considered offline" default:"4h" testDefault:"5m"`
}

// RepairThresholdOverrides override values for repair threshold.
type RepairThresholdOverrides struct {
	RepairOverrides
}

// RepairTargetOverrides override values for repair success target.
type RepairTargetOverrides struct {
	RepairOverrides
}

// RepairOverrides is a configuration struct that contains a list of  override repair
// values for various given RS combinations of k/o/n (min/success/total).
//
// Can be used as a flag.
type RepairOverrides struct {
	Values map[int]int
}

// Type implements pflag.Value.
func (RepairOverrides) Type() string { return "checker.RepairOverrides" }

// String is required for pflag.Value. It is a comma separated list of RepairOverride configs.
func (ros *RepairOverrides) String() string {
	var s strings.Builder
	i := 0
	for k, v := range ros.Values {
		if i > 0 {
			s.WriteString(",")
		}
		_, _ = fmt.Fprintf(&s, "%d-%d", k, v)
		i++
	}
	return s.String()
}

// Set sets the value from a string in the format "k-override,k-override,...".
func (ros *RepairOverrides) Set(s string) error {
	roStrings := strings.Split(s, ",")
	ros.Values = make(map[int]int, len(roStrings))
	for _, roString := range roStrings {
		roString = strings.TrimSpace(roString)
		if roString == "" {
			continue
		}
		parts := strings.Split(roString, "-")
		if len(parts) != 2 {
			return fmt.Errorf("invalid repair override value %q", s)
		}
		key, err := strconv.Atoi(strings.Split(parts[0], "/")[0]) // backwards compat
		if err != nil {
			return fmt.Errorf("invalid repair override value %q: %w", s, err)
		}
		if key <= 0 {
			return fmt.Errorf("invalid k, must be at least 1: %d", key)
		}
		val, err := strconv.Atoi(parts[1])
		if err != nil {
			return fmt.Errorf("invalid repair override value %q: %w", s, err)
		}
		if existingVal, exists := ros.Values[key]; exists && existingVal != val {
			return fmt.Errorf("key %d defined twice with different values: %q", key, s)
		}
		if val < key {
			return fmt.Errorf("key %d defined with value lower than min: %q", key, s)
		}
		ros.Values[key] = val
	}
	return nil
}

// GetOverrideValuePB returns the override value for a pb RS scheme if it exists, or 0 otherwise.
func (ros *RepairOverrides) GetOverrideValuePB(rs *pb.RedundancyScheme) int32 {
	return int32(ros.Values[int(rs.MinReq)])
}

// GetOverrideValue returns the override value for an RS scheme if it exists, or 0 otherwise.
func (ros *RepairOverrides) GetOverrideValue(rs storj.RedundancyScheme) int32 {
	return int32(ros.Values[int(rs.RequiredShares)])
}
