// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package piecelist

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase/rangedloop"
)

// Config is the configuration for the piece list observer.
type Config struct {
	TargetDir string `help:"directory to collect csv files with piece IDs" default:"."`
	NodeID    string `help:"node ID of the Storagenode to collect piece IDs for"`
}

// PieceList is an observer that collects piece IDs for a node.
type PieceList struct {
	mu        sync.Mutex
	counter   int
	outputDir string
	nodeID    storj.NodeID
}

var _ rangedloop.Observer = &PieceList{}

// NewPieceList creates a new piece list observer.
func NewPieceList(cfg Config) (*PieceList, error) {
	if cfg.NodeID == "" {
		return nil, errs.New("node ID is required")
	}
	nodeID, err := storj.NodeIDFromString(cfg.NodeID)
	if err != nil {
		return nil, err
	}
	return &PieceList{
		outputDir: cfg.TargetDir,
		nodeID:    nodeID,
	}, nil
}

// Start is called when the loop starts.
func (p *PieceList) Start(ctx context.Context, time time.Time) error {
	return nil
}

// Fork is called when the loop forks.
func (p *PieceList) Fork(ctx context.Context) (rangedloop.Partial, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	outputFile := filepath.Join(p.outputDir, fmt.Sprintf("pieces-%s-%d.csv", p.nodeID, p.counter))
	p.counter++
	destination, err := os.Create(outputFile)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &Fork{
		destination: destination,
		nodeID:      p.nodeID,
	}, nil
}

// Join is called when the partial is done.
func (p *PieceList) Join(ctx context.Context, partial rangedloop.Partial) error {
	return partial.(*Fork).destination.Close()
}

// Finish is called when the loop finishes.
func (p *PieceList) Finish(ctx context.Context) error {
	return nil
}

// Fork is a partial implementation that writes pieces to a file.
type Fork struct {
	destination *os.File
	nodeID      storj.NodeID
}

var _ rangedloop.Partial = &Fork{}

// Process is called for each segment.
func (f *Fork) Process(ctx context.Context, segments []rangedloop.Segment) error {
	for _, segment := range segments {

		// skip expired segments
		if segment.Expired(time.Now()) {
			continue
		}

		pieceNumber := -1
		for _, piece := range segment.Pieces {
			if piece.StorageNode == f.nodeID {
				pieceNumber = int(piece.Number)
				break
			}
		}

		if pieceNumber != -1 {
			_, err := f.destination.WriteString(fmt.Sprintf("%s,%d,%d\n", segment.StreamID, segment.Position.Encode(), pieceNumber))
			if err != nil {
				return errs.Wrap(err)
			}
		}
	}
	return nil
}
