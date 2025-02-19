// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cleanup

import (
	"context"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/filestore"
	"storj.io/storj/storagenode/piecestore"
)

// DeleteEmpty chore deletes the remaining empty blobstore directories.
type DeleteEmpty struct {
	rootDir string
	log     *zap.Logger
	blobs   blobstore.Blobs
}

// NewDeleteEmpty creates a new DeleteEmpty.
func NewDeleteEmpty(log *zap.Logger, config piecestore.OldConfig, blobs blobstore.Blobs) *DeleteEmpty {
	return &DeleteEmpty{
		rootDir: filepath.Join(config.Path, "blobs"),
		log:     log,
		blobs:   blobs,
	}
}

// Delete deletes old empty / zero sized dirs/files.
func (d *DeleteEmpty) Delete(ctx context.Context) error {
	namespaces, err := d.blobs.ListNamespaces(ctx)
	if err != nil {
		return err
	}
	for _, ns := range namespaces {
		satelliteID, err := storj.NodeIDFromBytes(ns)
		if err != nil {
			d.log.Error("Invalid namespace", zap.Binary("namespace", ns), zap.Error(err))
			continue
		}
		namespaceDir := filepath.Join(d.rootDir, filestore.PathEncoding.EncodeToString(satelliteID.Bytes()))

		prefixDirs, err := os.ReadDir(namespaceDir)
		if err != nil {
			d.log.Error("Prefix dirs couldn't be listed under namespace", zap.Binary("namespace", ns), zap.String("dir", namespaceDir), zap.Error(err))
			continue
		}
		for _, prefixDir := range prefixDirs {
			fileCount := 0
			if !prefixDir.IsDir() {
				continue
			}
			dirPath := filepath.Join(namespaceDir, prefixDir.Name())
			files, err := os.ReadDir(dirPath)
			if err != nil {
				d.log.Error("Files couldn't be listed under prefixDir", zap.Binary("namespace", ns), zap.String("dir", dirPath), zap.Error(err))
				continue
			}
			for _, pieceFile := range files {
				if pieceFile.IsDir() {
					continue
				}
				info, err := pieceFile.Info()
				if err == nil && info.Size() == 0 {
					filePath := filepath.Join(dirPath, info.Name())
					d.log.Info("Deleting zero sized file", zap.String("file", filePath))
					err := os.Remove(filePath)
					if err != nil {
						d.log.Warn("Couldn't delete zero sized file", zap.String("file", filePath), zap.Error(err))
					}
					continue
				}
				fileCount++

				// looks like our directories are not empty, yet. Better to ignore this job.
				if fileCount > 3 {
					break
				}
			}
			if fileCount == 0 {
				d.log.Info("Deleting empty directory", zap.String("dir", dirPath))
				err := os.Remove(dirPath)
				if err != nil {
					d.log.Warn("Couldn't delete directory", zap.String("dir", dirPath), zap.Error(err))
				}
			}
			if fileCount > 3 {
				break
			}
		}
	}
	return nil
}
