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
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/satellitedb"
)

var headers = []string{
	"userAgent",
	"projectID",
	"bucketName",
	"gbHours",
	"segmentHours",
	"objectHours",
	"hours",
	"gbEgress",
}

// GenerateAttributionCSV creates a report with.
func GenerateAttributionCSV(ctx context.Context, database string, start time.Time, end time.Time, output io.Writer) error {
	log := zap.L().Named("db")
	db, err := satellitedb.Open(ctx, log, database, satellitedb.Options{ApplicationName: "satellite-attribution"})
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

	w := csv.NewWriter(output)
	defer func() {
		w.Flush()
	}()

	if err := w.Write(headers); err != nil {
		return errs.Wrap(err)
	}

	for _, row := range rows {
		record, err := csvRowToStringSlice(row)
		if err != nil {
			return errs.Wrap(err)
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

func csvRowToStringSlice(p *attribution.CSVRow) ([]string, error) {
	projectID, err := uuid.FromBytes(p.ProjectID)
	if err != nil {
		return nil, errs.New("Invalid Project ID")
	}
	gbHours := memory.Size(p.ByteHours).GB()
	egressGBData := memory.Size(p.EgressData).GB()
	record := []string{
		string(p.UserAgent),
		projectID.String(),
		string(p.BucketName),
		strconv.FormatFloat(gbHours, 'f', 4, 64),
		strconv.FormatFloat(p.SegmentHours, 'f', 4, 64),
		strconv.FormatFloat(p.ObjectHours, 'f', 4, 64),
		strconv.FormatFloat(float64(p.Hours), 'f', 4, 64),
		strconv.FormatFloat(egressGBData, 'f', 4, 64),
	}
	return record, nil
}
