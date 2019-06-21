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

	"storj.io/storj/internal/memory"
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
		"bucketName",
		"byte-hours:Remote",
		"byte-hours:Inline",
		"bytes:BWEgress",
	}
	if err := w.Write(headers); err != nil {
		return err
	}

	for _, row := range rows {
		record, err := csvRowToStringSlice(row)
		if err != nil {
			return err
		}
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

func csvRowToStringSlice(p *attribution.ValueAttributionRow) ([]string, error) {
	projectID, err := bytesToUUID(p.ProjectID)
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

// bytesToUUID is used to convert []byte to UUID
func bytesToUUID(data []byte) (uuid.UUID, error) {
	var id uuid.UUID

	copy(id[:], data)
	if len(id) != len(data) {
		return uuid.UUID{}, errs.New("Invalid uuid")
	}

	return id, nil
}
