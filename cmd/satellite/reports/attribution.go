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

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/satellitedb"
)

// GenerateValueAttributionCSV creates a report with
func GenerateValueAttributionCSV(ctx context.Context, database string, partnerID uuid.UUID, start time.Time, end time.Time, output io.Writer) error {
	db, err := satellitedb.New(zap.L().Named("db"), database)
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	rows, err := db.Attribution().QueryValueAttribution(ctx, partnerID, start, end)
	if err != nil {
		return err
	}

	w := csv.NewWriter(output)
	headers := []string{
		"projectID",
		"bucketID",
		"byte-hours:Remote",
		"byte-hours:Inline",
		"bytes:BWEgress",
	}
	if err := w.Write(headers); err != nil {
		return err
	}

	for _, row := range rows {
		record := csvRowToStringSlice(row)
		if err := w.Write(record); err != nil {
			return err
		}
	}
	if err := w.Error(); err != nil {
		return err
	}
	w.Flush()
	if output != os.Stdout {
		fmt.Println("Generated node usage report for partner attribution")
	}
	return err
}

func csvRowToStringSlice(p *attribution.ValueAttributionRow) []string {
	record := []string{
		p.ProjectID,
		p.BucketID,
		//p.UserID,
		strconv.FormatFloat(p.AtRestData, 'f', 2, 64),
		strconv.FormatFloat(p.InlineData, 'f', 2, 64),
		strconv.FormatInt(p.EgressData, 10),
		//strconv.FormatInt(p.IngressData, 10),
		//p.LastUpdated.Format("2006-01-02"),
	}
	return record
}
