// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"time"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// PieceLocator specifies all information necessary to look up a particular piece
// on a particular satellite.
type PieceLocator struct {
	StreamID uuid.UUID
	Position metabase.SegmentPosition
	NodeID   storj.NodeID
	PieceNum int
}

// ReverificationJob represents a job as received from the reverification
// audit queue.
type ReverificationJob struct {
	Locator       PieceLocator
	InsertedAt    time.Time
	ReverifyCount int
}
