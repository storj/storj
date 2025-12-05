// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedbtest

import (
	"archive/zip"
	"bytes"
	"context"
	_ "embed"
	"io"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/storagenode/storagenodedb"
)

//go:generate go run ./gensnapshot testdata/snapshot.zip
//go:embed testdata/snapshot.zip
var snapshotZip []byte

// OpenNew opens a new storage node database pre-populated with a newly
// initialized and migrated database snapshot.
func OpenNew(ctx context.Context, log *zap.Logger, config storagenodedb.Config) (*storagenodedb.DB, error) {
	db, err := storagenodedb.OpenNew(ctx, log, config)
	if err != nil {
		return nil, err
	}
	if err := deploySnapshot(db.DBDirectory()); err != nil {
		return nil, err
	}
	return db, nil
}

func deploySnapshot(storageDir string) error {
	zipReader, err := zip.NewReader(bytes.NewReader(snapshotZip), int64(len(snapshotZip)))
	if err != nil {
		return errs.Wrap(err)
	}

	for _, f := range zipReader.File {
		rc, err := f.Open()
		if err != nil {
			return errs.Wrap(err)
		}
		data, err := io.ReadAll(rc)
		if err != nil {
			return errs.Wrap(err)
		}
		if err := os.WriteFile(filepath.Join(storageDir, f.Name), data, 0644); err != nil {
			return errs.Wrap(err)
		}
		if err := rc.Close(); err != nil {
			return errs.Wrap(err)
		}
	}
	return nil
}
