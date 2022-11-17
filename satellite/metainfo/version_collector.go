// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"fmt"
	"sort"
	"strings"

	"github.com/blang/semver"
	"github.com/spacemonkeygo/monkit/v3"
	"go.uber.org/zap"

	"storj.io/common/useragent"
)

const uplinkProduct = "uplink"

type transfer string

const upload = transfer("upload")
const download = transfer("download")

var knownUserAgents = []string{
	"rclone", "gateway-st", "gateway-mt", "linksharing", "uplink-cli", "transfer-sh", "filezilla", "duplicati",
	"comet", "orbiter", "uplink-php", "nextcloud", "aws-cli", "ipfs-go-ds-storj", "storj-downloader",
}

type versionOccurrence struct {
	Product string
	Version string
	Method  string
}

type versionCollector struct {
	log *zap.Logger
}

func newVersionCollector(log *zap.Logger) *versionCollector {
	return &versionCollector{
		log: log,
	}
}

func (vc *versionCollector) collect(useragentRaw []byte, method string) {
	if len(useragentRaw) == 0 {
		return
	}

	entries, err := useragent.ParseEntries(useragentRaw)
	if err != nil {
		vc.log.Warn("unable to collect uplink version", zap.Error(err))
		mon.Meter("user_agents", monkit.NewSeriesTag("user_agent", "unparseable")).Mark(1)
		return
	}

	// foundProducts tracks potentially multiple noteworthy products names from the user-agent
	var foundProducts []string
	for _, entry := range entries {
		product := strings.ToLower(entry.Product)
		if product == uplinkProduct {
			vo := versionOccurrence{Product: product, Version: entry.Version, Method: method}
			vc.sendUplinkMetric(vo)
		} else if contains(knownUserAgents, product) && !contains(foundProducts, product) {
			foundProducts = append(foundProducts, product)
		}
	}

	if len(foundProducts) > 0 {
		sort.Strings(foundProducts)
		// concatenate all known products for this metric, EG "gateway-mt + rclone"
		mon.Meter("user_agents", monkit.NewSeriesTag("user_agent", strings.Join(foundProducts, " + "))).Mark(1)
	} else { // lets keep also general value for user agents with no known product
		mon.Meter("user_agents", monkit.NewSeriesTag("user_agent", "other")).Mark(1)
	}
}

func (vc *versionCollector) sendUplinkMetric(vo versionOccurrence) {
	if vo.Version == "" {
		vo.Version = "unknown"
	} else {
		// use only minor to avoid using too many resources and
		// minimize risk of abusing by sending lots of different versions
		semVer, err := semver.ParseTolerant(vo.Version)
		if err != nil {
			vc.log.Warn("invalid uplink library user agent version", zap.String("version", vo.Version), zap.Error(err))
			return
		}

		// keep number of possible versions very limited
		if semVer.Major != 1 || semVer.Minor > 30 {
			vc.log.Warn("invalid uplink library user agent version", zap.String("version", vo.Version), zap.Error(err))
			return
		}

		vo.Version = fmt.Sprintf("v%d.%d", 1, semVer.Minor)
	}

	mon.Meter("uplink_versions", monkit.NewSeriesTag("version", vo.Version), monkit.NewSeriesTag("method", vo.Method)).Mark(1)
}

func (vc *versionCollector) collectTransferStats(useragentRaw []byte, transfer transfer, transferSize int) {
	entries, err := useragent.ParseEntries(useragentRaw)
	if err != nil {
		vc.log.Warn("unable to collect transfer statistics", zap.Error(err))
		mon.Meter("user_agents_transfer_stats", monkit.NewSeriesTag("user_agent", "unparseable"), monkit.NewSeriesTag("type", string(transfer))).Mark(transferSize)
		return
	}

	// foundProducts tracks potentially multiple noteworthy products names from the user-agent
	var foundProducts []string
	for _, entry := range entries {
		product := strings.ToLower(entry.Product)
		if contains(knownUserAgents, product) && !contains(foundProducts, product) {
			foundProducts = append(foundProducts, product)
		}
	}

	if len(foundProducts) > 0 {
		sort.Strings(foundProducts)
		// concatenate all known products for this metric, EG "gateway-mt + rclone"
		mon.Meter("user_agents_transfer_stats", monkit.NewSeriesTag("user_agent", strings.Join(foundProducts, " + ")), monkit.NewSeriesTag("type", string(transfer))).Mark(transferSize)
	} else { // lets keep also general value for user agents with no known product
		mon.Meter("user_agents_transfer_stats", monkit.NewSeriesTag("user_agent", "other"), monkit.NewSeriesTag("type", string(transfer))).Mark(transferSize)
	}
}

// contains returns true if the given string is contained in the given slice.
func contains(slice []string, testValue string) bool {
	for _, sliceValue := range slice {
		if sliceValue == testValue {
			return true
		}
	}
	return false
}
