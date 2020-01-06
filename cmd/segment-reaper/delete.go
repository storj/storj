// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"io"
	"os"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
	"storj.io/storj/pkg/process"
	"storj.io/storj/satellite/metainfo"
	"storj.io/storj/storage"
)

var (
	errKnown = errs.Class("known delete error")

	deleteCmd = &cobra.Command{
		Use:   "delete input_file.csv [flags]",
		Short: "Deletes zombie segments from DB",
		Args:  cobra.ExactArgs(1),
		RunE:  cmdDelete,
	}

	deleteCfg struct {
		DatabaseURL string `help:"the database connection string to use" default:"postgres://"`
		DryRun      bool   `help:"with this option no deletion will be done, only printing results" default:"false"`
	}
)

func init() {
	rootCmd.AddCommand(deleteCmd)

	process.Bind(deleteCmd, &deleteCfg)
}

func cmdDelete(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	log := zap.L()
	db, err := metainfo.NewStore(log.Named("pointerdb"), deleteCfg.DatabaseURL)
	if err != nil {
		return errs.New("error connecting database: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	inputFile, err := os.Open(args[0])
	if err != nil {
		return errs.New("error opening input file: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, inputFile.Close())
	}()

	csvReader := csv.NewReader(inputFile)
	csvReader.FieldsPerRecord = 5
	csvReader.ReuseRecord = true

	segmentsDeleted := 0
	segmentsErrored := 0
	segmentsSkipped := 0
	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error("error while reading record", zap.Error(err))
			continue
		}

		projectID := record[0]
		segmentIndex := record[1]
		bucketName := record[2]
		encodedPath := record[3]
		creationDateFromReport, err := time.Parse(time.RFC3339Nano, record[4])
		if err != nil {
			log.Error("error while parsing date", zap.Error(err))
			continue
		}

		encryptedPath, err := base64.StdEncoding.DecodeString(encodedPath)
		if err != nil {
			log.Error("error while decoding encrypted path", zap.Error(err))
			continue
		}

		path := storj.JoinPaths(projectID, segmentIndex, bucketName, string(encryptedPath))
		rawPath := storj.JoinPaths(projectID, segmentIndex, bucketName, encodedPath)

		err = deleteSegment(ctx, db, path, creationDateFromReport, deleteCfg.DryRun)
		if err != nil {
			if errKnown.Has(err) {
				segmentsSkipped++
			} else {
				segmentsErrored++
			}
			log.Error("error while deleting segment", zap.String("path", rawPath), zap.Error(err))
			continue
		}

		log.Debug("segment deleted", zap.String("path", rawPath))
		segmentsDeleted++
	}

	log.Info("summary", zap.Int("deleted", segmentsDeleted), zap.Int("skipped", segmentsSkipped), zap.Int("errored", segmentsErrored))

	return nil
}

func deleteSegment(ctx context.Context, db metainfo.PointerDB, path string, creationDate time.Time, dryRun bool) error {
	pointerBytes, err := db.Get(ctx, []byte(path))
	if err != nil {
		if storage.ErrKeyNotFound.Has(err) {
			return errKnown.New("segment already deleted by user: %+v", err)
		}
		return err
	}

	pointer := &pb.Pointer{}
	err = proto.Unmarshal(pointerBytes, pointer)
	if err != nil {
		return err
	}

	// check if pointer has been replaced
	if !pointer.GetCreationDate().Equal(creationDate) {
		// pointer has been replaced since detection, do not delete it.
		return errKnown.New("segment won't be deleted, create date mismatch: %s -> %s", pointer.GetCreationDate(), creationDate)
	}

	if !dryRun {
		// delete the pointer using compare-and-swap
		err = db.CompareAndSwap(ctx, []byte(path), pointerBytes, nil)
		if err != nil {
			if storage.ErrValueChanged.Has(err) {
				// race detected while deleting the pointer, do not try deleting it again.
				return errKnown.New("segment won't be deleted, race detected while deleting the pointer: %+v", err)
			}
			return err
		}
	}

	return nil
}
