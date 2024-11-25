// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/shared/modular"
)

type RunOnceConfig struct {
	Segment string `help:"segment and position in the formant of stream-id/position" default:""`
}

// RunOnce is a helper to run the ranged loop only once.
type RunOnce struct {
	verifier *Verifier
	stop     *modular.StopTrigger
	log      *zap.Logger
	segment  Segment
}

// NewRunOnce creates a new RunOnce.
func NewRunOnce(log *zap.Logger, verifier *Verifier, stop *modular.StopTrigger, r RunOnceConfig) (*RunOnce, error) {
	streamID, position, err := ParseSegmentPosition(r.Segment)
	return &RunOnce{
		log:      log,
		verifier: verifier,
		stop:     stop,
		segment: Segment{
			StreamID: streamID,
			Position: position,
		},
	}, err
}

// Run executes ranged loop only once.
func (r *RunOnce) Run(ctx context.Context) error {
	defer func() {
		r.stop.Cancel()
	}()

	report, err := r.verifier.Verify(ctx, r.segment, nil)
	if err != nil {
		return err
	}
	fmt.Println("FAILED")
	for _, nodeID := range report.Fails {
		fmt.Println("   ", nodeID)
	}
	fmt.Println("SUCCESS")
	for _, nodeID := range report.Successes {
		fmt.Println("   ", nodeID)
	}
	fmt.Println("UNKNOWN")
	for _, nodeID := range report.Unknown {
		fmt.Println("   ", nodeID)
	}
	return nil
}

// ParseSegmentPosition parse segment position from segment/pos format
func ParseSegmentPosition(i string) (uuid.UUID, metabase.SegmentPosition, error) {
	sp := metabase.SegmentPosition{}
	parts := strings.Split(i, "/")

	if len(parts) > 1 {
		part, err := strconv.Atoi(parts[1])
		if err != nil {
			return uuid.UUID{}, metabase.SegmentPosition{}, err
		}
		sp = metabase.SegmentPositionFromEncoded(uint64(part))
	}
	su, err := ParseUUID(parts[0])
	if err != nil {
		return uuid.UUID{}, metabase.SegmentPosition{}, err
	}
	return su, sp, nil
}

func ParseUUID(id string) (uuid.UUID, error) {
	if id[0] == '#' {
		sid, _ := uuid.New()
		decoded, err := hex.DecodeString(id[1:])
		if err != nil {
			return uuid.UUID{}, err
		}
		copy(sid[:], decoded)
		fmt.Println(sid.String())
		return sid, nil
	}
	if !strings.Contains(id, "-") {
		id = id[0:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:]
		fmt.Println(id)
	}
	return uuid.FromString(id)
}
