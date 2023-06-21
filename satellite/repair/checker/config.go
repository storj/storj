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

	ReliabilityCacheStaleness time.Duration   `help:"how stale reliable node cache can be" releaseDefault:"5m" devDefault:"5m" testDefault:"1m"`
	RepairOverrides           RepairOverrides `help:"comma-separated override values for repair threshold in the format k/o/n-override (min/optimal/total-override)" releaseDefault:"29/80/110-52,29/80/95-52,29/80/130-52" devDefault:""`
	// Node failure rate is an estimation based on a 6 hour checker run interval (4 checker iterations per day), a network of about 9200 nodes, and about 2 nodes churning per day.
	// This results in `2/9200/4 = 0.00005435` being the probability of any single node going down in the interval of one checker iteration.
	NodeFailureRate            float64 `help:"the probability of a single node going down within the next checker iteration" default:"0.00005435" `
	RepairQueueInsertBatchSize int     `help:"Number of damaged segments to buffer in-memory before flushing to the repair queue" default:"100" `
	DoDeclumping               bool    `help:"Treat pieces on the same network as in need of repair" default:"false"`
	DoPlacementCheck           bool    `help:"Treat pieces out of segment placement as in need of repair" default:"true"`
}

// RepairOverride is a configuration struct that contains an override repair
// value for a given RS k/o/n (min/success/total).
//
// Can be used as a flag.
type RepairOverride struct {
	Min      int
	Success  int
	Total    int
	Override int32
}

// Type implements pflag.Value.
func (RepairOverride) Type() string { return "checker.RepairOverride" }

// String is required for pflag.Value.
func (ro *RepairOverride) String() string {
	return fmt.Sprintf("%d/%d/%d-%d",
		ro.Min,
		ro.Success,
		ro.Total,
		ro.Override)
}

// Set sets the value from a string in the format k/o/n-override (min/optimal/total-repairOverride).
func (ro *RepairOverride) Set(s string) error {
	// Split on dash. Expect two items. First item is RS numbers. Second item is Override.
	info := strings.Split(s, "-")
	if len(info) != 2 {
		return Error.New("Invalid default repair override config (expect format k/o/n-override, got %s)", s)
	}
	rsNumbersString := info[0]
	overrideString := info[1]

	// Split on forward slash. Expect exactly three positive non-decreasing integers.
	rsNumbers := strings.Split(rsNumbersString, "/")
	if len(rsNumbers) != 3 {
		return Error.New("Invalid default RS numbers (wrong size, expect 3): %s", rsNumbersString)
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

	ro.Min = values[0]
	ro.Success = values[1]
	ro.Total = values[2]

	// Attempt to parse "-override" part of config.
	override, err := strconv.Atoi(overrideString)
	if err != nil {
		return Error.New("Invalid override value (should be valid integer): %s, %w", overrideString, err)
	}
	if override < ro.Min || override >= ro.Success {
		return Error.New("Invalid override value (should meet criteria min <= override < success). Min: %d, Override: %d, Success: %d.", ro.Min, override, ro.Success)
	}
	ro.Override = int32(override)

	return nil
}

// RepairOverrides is a configuration struct that contains a list of  override repair
// values for various given RS combinations of k/o/n (min/success/total).
//
// Can be used as a flag.
type RepairOverrides struct {
	List []RepairOverride
}

// Type implements pflag.Value.
func (RepairOverrides) Type() string { return "checker.RepairOverrides" }

// String is required for pflag.Value. It is a comma separated list of RepairOverride configs.
func (ros *RepairOverrides) String() string {
	var s strings.Builder
	for i, ro := range ros.List {
		if i > 0 {
			s.WriteString(",")
		}
		s.WriteString(ro.String())
	}
	return s.String()
}

// Set sets the value from a string in the format "k/o/n-override,k/o/n-override,...".
func (ros *RepairOverrides) Set(s string) error {
	ros.List = nil
	roStrings := strings.Split(s, ",")
	for _, roString := range roStrings {
		roString = strings.TrimSpace(roString)
		if roString == "" {
			continue
		}
		newRo := RepairOverride{}
		err := newRo.Set(roString)
		if err != nil {
			return err
		}
		ros.List = append(ros.List, newRo)
	}
	return nil
}

// GetMap creates a RepairOverridesMap from the config.
func (ros *RepairOverrides) GetMap() RepairOverridesMap {
	newMap := RepairOverridesMap{
		overrideMap: make(map[string]int32),
	}
	for _, ro := range ros.List {
		key := getRepairOverrideKey(ro.Min, ro.Success, ro.Total)
		newMap.overrideMap[key] = ro.Override
	}
	return newMap
}

// RepairOverridesMap is derived from the RepairOverrides config, and is used for quickly retrieving
// repair override values.
type RepairOverridesMap struct {
	// map of "k/o/n" -> override value
	overrideMap map[string]int32
}

// GetOverrideValuePB returns the override value for a pb RS scheme if it exists, or 0 otherwise.
func (rom *RepairOverridesMap) GetOverrideValuePB(rs *pb.RedundancyScheme) int32 {
	key := getRepairOverrideKey(int(rs.MinReq), int(rs.SuccessThreshold), int(rs.Total))
	return rom.overrideMap[key]
}

// GetOverrideValue returns the override value for an RS scheme if it exists, or 0 otherwise.
func (rom *RepairOverridesMap) GetOverrideValue(rs storj.RedundancyScheme) int32 {
	key := getRepairOverrideKey(int(rs.RequiredShares), int(rs.OptimalShares), int(rs.TotalShares))
	return rom.overrideMap[key]
}

func getRepairOverrideKey(min, success, total int) string {
	return fmt.Sprintf("%d/%d/%d", min, success, total)
}
