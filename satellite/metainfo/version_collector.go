// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"strings"
	"sync"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/useragent"
)

const uplinkProduct = "uplink"

type versionOccurrence struct {
	Version string
	Method  string
}

type versionCollector struct {
	mu       sync.Mutex
	versions map[versionOccurrence]*monkit.Meter
}

func newVersionCollector() *versionCollector {
	return &versionCollector{
		versions: make(map[versionOccurrence]*monkit.Meter),
	}
}

func (vc *versionCollector) collect(useragentRaw []byte, method string) error {
	var meter *monkit.Meter

	version := "unknown"
	if len(useragentRaw) != 0 {
		entries, err := useragent.ParseEntries(useragentRaw)
		if err != nil {
			return errs.New("invalid user agent %q: %v", string(useragentRaw), err)
		}

		for _, entry := range entries {
			if strings.EqualFold(entry.Product, uplinkProduct) {
				version = entry.Version
				break
			}
		}
	}

	vo := versionOccurrence{
		Version: version,
		Method:  method,
	}

	vc.mu.Lock()
	meter, ok := vc.versions[vo]
	if !ok {
		meter = monkit.NewMeter(monkit.NewSeriesKey("uplink_versions").WithTag("version", version).WithTag("method", method))
		mon.Chain(meter)
		vc.versions[vo] = meter
	}
	vc.mu.Unlock()

	meter.Mark(1)
	return nil
}
