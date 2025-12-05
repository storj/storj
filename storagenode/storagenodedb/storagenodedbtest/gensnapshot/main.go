// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/storagenode/storagenodedb"
)

func main() {
	outFile := "snapshot.zip"
	if len(os.Args) > 1 {
		outFile = os.Args[1]
	}
	if err := run(context.Background(), outFile); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to generate snapshot database: %+v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, outFile string) error {
	log, err := zap.NewDevelopment()
	if err != nil {
		return errs.Wrap(err)
	}

	tempDir, err := os.MkdirTemp("", "")
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			log.Warn("Could not remove temp directory", zap.Error(err), zap.String("directory", tempDir))
		}
	}()

	cfg := storagenodedb.Config{
		Storage: tempDir,
		Info:    filepath.Join(tempDir, "piecestore.db"),
		Info2:   filepath.Join(tempDir, "info.db"),
		Pieces:  tempDir,

		TestingDisableWAL: true,
	}

	db, err := storagenodedb.OpenNew(ctx, log, cfg)
	if err != nil {
		return errs.Wrap(err)
	}

	err = db.MigrateToLatest(ctx)
	if err != nil {
		return errs.Wrap(err)
	}

	err = db.Close()
	if err != nil {
		return errs.Wrap(err)
	}

	matches, err := filepath.Glob(filepath.Join(tempDir, "*.db"))
	if err != nil {
		return errs.Wrap(err)
	}

	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)
	for _, match := range matches {
		data, err := os.ReadFile(match)
		if err != nil {
			return errs.Wrap(err)
		}
		w, err := zipWriter.Create(filepath.Base(match))
		if err != nil {
			return errs.Wrap(err)
		}
		_, err = io.Copy(w, bytes.NewReader(data))
		if err != nil {
			return errs.Wrap(err)
		}
	}
	if err := zipWriter.Close(); err != nil {
		return errs.Wrap(err)
	}

	if err := os.WriteFile(outFile, buf.Bytes(), 0644); err != nil {
		return errs.Wrap(err)
	}

	return nil
}
