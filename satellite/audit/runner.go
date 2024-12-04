// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/shared/modular"
)

// RunOnceConfig is the config for the RunOnce.
type RunOnceConfig struct {
	Segment string `help:"segment and position in the formant of stream-id/position" default:""`
}

// RunOnce is a helper to run the ranged loop only once.
type RunOnce struct {
	verifier *Verifier
	stop     *modular.StopTrigger
	log      *zap.Logger
	segment  string
}

// NewRunOnce creates a new RunOnce.
func NewRunOnce(log *zap.Logger, verifier *Verifier, stop *modular.StopTrigger, r RunOnceConfig) (*RunOnce, error) {
	return &RunOnce{
		log:      log,
		verifier: verifier,
		stop:     stop,
		segment:  r.Segment,
	}, nil
}

// Run executes ranged loop only once.
func (r *RunOnce) Run(ctx context.Context) error {
	defer func() {
		r.stop.Cancel()
	}()
	if _, err := os.Stat(r.segment); err == nil {
		in, err := os.Open(r.segment)
		if err != nil {
			return errs.Wrap(err)
		}
		defer func(in *os.File) {
			_ = in.Close()
		}(in)
		ci := csv.NewReader(in)
		for {
			record, err := ci.Read()
			if errs.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return errs.Wrap(err)
			}
			streamID, position, err := ParseSegmentPosition(fmt.Sprintf("%s/%s", record[0], record[1]))
			if err != nil {
				return errs.Wrap(err)
			}
			err = r.auditOne(ctx, streamID, position)
			if err != nil {
				r.log.Warn("Audit is failed", zap.Stringer("stream_id", streamID), zap.Uint64("position", position.Encode()), zap.Error(err))
			}
		}
		return nil
	}

	streamID, position, err := ParseSegmentPosition(r.segment)
	if err != nil {
		return err
	}
	err = r.auditOne(ctx, streamID, position)
	if err != nil {
		return err
	}

	return nil
}

func (r *RunOnce) auditOne(ctx context.Context, streamID uuid.UUID, position metabase.SegmentPosition) error {
	report, err := r.verifier.Verify(ctx, Segment{
		StreamID: streamID,
		Position: position,
	}, nil)
	if err != nil {
		return err
	}
	for _, result := range report.Fails {
		mon.Counter("audit_failure", monkit.NewSeriesTag("node", result.StorageNode.String())).Inc(1)
	}
	for _, result := range report.Unknown {
		mon.Counter("audit_unknown", monkit.NewSeriesTag("node", result.String())).Inc(1)
	}
	for _, result := range report.Successes {
		mon.Counter("audit_success", monkit.NewSeriesTag("node", result.String())).Inc(1)
	}

	return err
}

// ParseSegmentPosition parse segment position from segment/pos format.
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
	su, err := parseUUID(parts[0])
	if err != nil {
		return uuid.UUID{}, metabase.SegmentPosition{}, err
	}
	return su, sp, nil
}

// parse UUID from string, but allow to use both hex and `-` separated format.
func parseUUID(id string) (uuid.UUID, error) {
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
