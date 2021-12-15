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

	for _, entry := range entries {
		if strings.EqualFold(entry.Product, uplinkProduct) {
			vo := versionOccurrence{
				Product: entry.Product,
				Version: entry.Version,
				Method:  method,
			}

			vc.sendUplinkMetric(vo)
		} else {
			// for other user agents monitor only product
			product := entry.Product
			if product == "" {
				product = "unknown"
			}
			mon.Meter("user_agents", monkit.NewSeriesTag("user_agent", product)).Mark(1)
		}
	}

	return nil
}

func (vc *versionCollector) sendUplinkMetric(vo versionOccurrence) {
	if vo.Version == "" {
		vo.Version = "unknown"
	} else {
		// use only major and minor to avoid using too many resources and
		// minimize risk of abusing by sending lots of different versions
		semVer, err := semver.ParseTolerant(vo.Version)
		if err != nil {
			vc.log.Warn("invalid uplink library user agent version", zap.String("version", vo.Version), zap.Error(err))
			return
		}
		vo.Version = fmt.Sprintf("v%d.%d", semVer.Major, semVer.Minor)
	}

	mon.Meter("uplink_versions", monkit.NewSeriesTag("version", vo.Version), monkit.NewSeriesTag("method", vo.Method)).Mark(1)
}
