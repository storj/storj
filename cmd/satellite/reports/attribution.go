// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package reports

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/useragent"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/satellitedb"
)

var headers = []string{
	"userAgent",
	"gbHours",
	"segmentHours",
	"objectHours",
	"hours",
	"gbEgress",
}

// AttributionTotals is a map of attribution totals per user agent.
type AttributionTotals map[string]Total

// Total is the total attributable usage for a user agent over a period of time.
type Total struct {
	ByteHours    float64
	SegmentHours float64
	ObjectHours  float64
	BucketHours  float64
	BytesEgress  int64
}

// GenerateAttributionCSV creates a report with.
func GenerateAttributionCSV(ctx context.Context, database string, start time.Time, end time.Time, output io.Writer) error {
	log := zap.L().Named("attribution-report")
	db, err := satellitedb.Open(ctx, log.Named("db"), database, satellitedb.Options{ApplicationName: "satellite-attribution"})
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
		if err != nil {
			log.Error("Error closing satellite DB connection after retrieving partner value attribution data.", zap.Error(err))
		}
	}()

	rows, err := db.Attribution().QueryAllAttribution(ctx, start, end)
	if err != nil {
		return errs.Wrap(err)
	}

	partnerAttributionTotals := SumAttributionByUserAgent(rows, log)

	w := csv.NewWriter(output)
	defer func() {
		w.Flush()
	}()

	if err := w.Write(headers); err != nil {
		return errs.Wrap(err)
	}
	for userAgent, totals := range partnerAttributionTotals {
		gbHours := memory.Size(totals.ByteHours).GB()
		egressGBData := memory.Size(totals.BytesEgress).GB()
		record := []string{
			userAgent,
			strconv.FormatFloat(gbHours, 'f', 4, 64),
			strconv.FormatFloat(totals.SegmentHours, 'f', 4, 64),
			strconv.FormatFloat(totals.ObjectHours, 'f', 4, 64),
			strconv.FormatFloat(totals.BucketHours, 'f', 4, 64),
			strconv.FormatFloat(egressGBData, 'f', 4, 64),
		}
		if err := w.Write(record); err != nil {
			return errs.Wrap(err)
		}
	}
	if err := w.Error(); err != nil {
		return errs.Wrap(err)
	}

	if output != os.Stdout {
		fmt.Println("Generated report for partner attribution")
	}
	return errs.Wrap(err)
}

// SumAttributionByUserAgent sums all bucket attribution by the first entry in the user agent.
func SumAttributionByUserAgent(rows []*attribution.BucketUsage, log *zap.Logger) AttributionTotals {
	attributionTotals := make(map[string]Total)
	userAgentParseFailures := make(map[string]bool)

	for _, row := range rows {
		userAgentEntries, err := useragent.ParseEntries(row.UserAgent)
		// also check the length of user agent for sanity.
		// If the length of user agent is zero the parse method will not return an error.
		if err != nil || len(row.UserAgent) == 0 {
			if _, ok := userAgentParseFailures[string(row.UserAgent)]; !ok {
				userAgentParseFailures[string(row.UserAgent)] = true
				log.Error("error while parsing user agent", zap.String("user agent", string(row.UserAgent)), zap.Error(err))
			}
			continue
		}

		userAgent := strings.ToLower(userAgentEntries[0].Product)

		if _, ok := attributionTotals[userAgent]; !ok {
			attributionTotals[userAgent] = Total{
				ByteHours:    row.ByteHours,
				SegmentHours: row.SegmentHours,
				ObjectHours:  row.ObjectHours,
				BucketHours:  float64(row.Hours),
				BytesEgress:  row.EgressData,
			}
		} else {
			partnerTotal := attributionTotals[userAgent]

			partnerTotal.ByteHours += row.ByteHours
			partnerTotal.SegmentHours += row.SegmentHours
			partnerTotal.ObjectHours += row.ObjectHours
			partnerTotal.BucketHours += float64(row.Hours)
			partnerTotal.BytesEgress += row.EgressData

			attributionTotals[userAgent] = partnerTotal
		}
	}
	return attributionTotals
}
