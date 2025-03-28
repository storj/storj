// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package rangedloop

import (
	"context"
	"time"

	"go.uber.org/zap/zapcore"

	"storj.io/storj/satellite/metabase"
)

// Segment contains information about segment metadata which will be received by observers.
type Segment metabase.LoopSegmentEntry

// Inline returns true if segment is inline.
func (s Segment) Inline() bool {
	return (s.Redundancy.IsZero() && len(s.Pieces) == 0) || s.RootPieceID.IsZero()
}

// Expired checks if segment expired relative to now.
func (s *Segment) Expired(now time.Time) bool {
	return s.ExpiresAt != nil && s.ExpiresAt.Before(now)
}

// PieceSize returns calculated piece size for segment.
func (s Segment) PieceSize() int64 {
	return s.Redundancy.PieceSize(int64(s.EncryptedSize))
}

// MarshalLogObject implements zapcore.ObjectMarshaler.
func (s Segment) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("StreamID", s.StreamID.String())
	enc.AddUint64("Position", s.Position.Encode())
	enc.AddUint16("Placement", uint16(s.Placement))
	return nil
}

// Observer subscribes to the parallel segment loop.
// It is intended that a naïve implementation is threadsafe.
type Observer interface {
	// Start is called at the beginning of each segment loop.
	Start(context.Context, time.Time) error

	// Fork creates a Partial to process a chunk of all the segments. It is
	// called after Start. It is not called concurrently.
	Fork(context.Context) (Partial, error)

	// Join is called for each partial returned by Fork. This gives the
	// opportunity to merge the output like in a reduce step. It will be called
	// before Finish. It is not called concurrently.
	Join(context.Context, Partial) error

	// Finish is called after all segments are processed by all observers.
	Finish(context.Context) error
}

// Partial processes a part of the total range of segments.
type Partial interface {
	// Process is called repeatedly with batches of segments.
	// It is not called concurrently on the same instance.
	Process(context.Context, []Segment) error
}
