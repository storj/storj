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

	"storj.io/common/memory"
	"storj.io/storj/private/dbutil"
	"storj.io/storj/satellite/attribution"
	"storj.io/storj/satellite/satellitedb"
)

var headers = []string{
	"projectID",
	"bucketName",
	"byte-hours:Remote",
	"byte-hours:Inline",
	"bytes:BWEgress",
}

// GenerateAttributionCSV creates a report with
func GenerateAttributionCSV(ctx context.Context, database string, partnerID uuid.UUID, start time.Time, end time.Time, output io.Writer) error {
	log := zap.L().Named("db")
	db, err := satellitedb.New(log, database)
	if err != nil {
		return errs.New("error connecting to master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
		if err != nil {
			log.Sugar().Errorf("error closing satellite DB connection after retrieving partner value attribution data: %+v", err)
		}
	}()

	rows, err := db.Attribution().QueryAttribution(ctx, partnerID, start, end)
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
	projectID, err := dbutil.BytesToUUID(p.ProjectID)
	if err != nil {
		return nil, errs.New("Invalid Project ID")
	}
	remoteGBPerHour := memory.Size(p.RemoteBytesPerHour).GB()
	inlineGBPerHour := memory.Size(p.InlineBytesPerHour).GB()
	egressGBData := memory.Size(p.EgressData).GB()
	record := []string{
		projectID.String(),
		string(p.BucketName),
		strconv.FormatFloat(remoteGBPerHour, 'f', 4, 64),
		strconv.FormatFloat(inlineGBPerHour, 'f', 4, 64),
		strconv.FormatFloat(egressGBData, 'f', 4, 64),
	}
	return record, nil
}
