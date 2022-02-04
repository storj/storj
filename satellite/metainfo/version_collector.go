// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/useragent"
)

const uplinkProduct = "uplink"

var knownUserAgents = []string{
	"rclone", "gateway-st", "gateway-mt", "linksharing", "uplink-cli", "transfer-sh", "filezilla", "duplicati",
	"comet", "orbiter",
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

func (vc *versionCollector) collect(useragentRaw []byte, method string) error {
	if len(useragentRaw) == 0 {
		return nil
	}

	entries, err := useragent.ParseEntries(useragentRaw)
	if err != nil {
		return errs.New("invalid user agent %q: %v", string(useragentRaw), err)
	}

	foundProduct := false
	for _, entry := range entries {
		if strings.EqualFold(entry.Product, uplinkProduct) {
			vo := versionOccurrence{
				Product: entry.Product,
				Version: entry.Version,
				Method:  method,
			}

			vc.sendUplinkMetric(vo)
		} else if knownUserAgent(entry.Product) {
			// for known user agents monitor only product
			mon.Meter("user_agents", monkit.NewSeriesTag("user_agent", strings.ToLower(entry.Product))).Mark(1)
			foundProduct = true
		}
	}
	if !foundProduct { // lets keep also general value for other user agents
		mon.Meter("user_agents", monkit.NewSeriesTag("user_agent", "other")).Mark(1)
	}

	return nil
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

func knownUserAgent(userAgent string) bool {
	for _, knownUserAgent := range knownUserAgents {
		if strings.EqualFold(userAgent, knownUserAgent) {
			return true
		}
	}
	return false
}
