// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"sync/atomic"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
)

var mon = monkit.Package()

// Error is global error class.
var Error = errs.Class("segment-verify")

// Metabase defines implementation dependencies we need from metabase.
type Metabase interface {
	ConvertNodesToAliases(ctx context.Context, nodeID []storj.NodeID) ([]metabase.NodeAlias, error)
	ConvertAliasesToNodes(ctx context.Context, aliases []metabase.NodeAlias) ([]storj.NodeID, error)
	GetSegmentByPosition(ctx context.Context, opts metabase.GetSegmentByPosition) (segment metabase.Segment, err error)
	ListVerifySegments(ctx context.Context, opts metabase.ListVerifySegments) (result metabase.ListVerifySegmentsResult, err error)
}

// Verifier verifies a batch of segments.
type Verifier interface {
	Verify(ctx context.Context, target storj.NodeURL, segments []*Segment, ignoreThrottle bool) error
}

// Overlay is used to fetch information about nodes.
type Overlay interface {
	// Get looks up the node by nodeID
	Get(ctx context.Context, nodeID storj.NodeID) (*overlay.NodeDossier, error)
	SelectAllStorageNodesDownload(ctx context.Context, onlineWindow time.Duration, asOf overlay.AsOfSystemTimeConfig) ([]*overlay.SelectedNode, error)
}

// SegmentWriter allows writing segments to some output.
type SegmentWriter interface {
	Write(ctx context.Context, segments []*Segment) error
	Close() error
}

// ServiceConfig contains configurable options for Service.
type ServiceConfig struct {
	NotFoundPath      string `help:"segments not found on storage nodes" default:"segments-not-found.csv"`
	RetryPath         string `help:"segments unable to check against satellite" default:"segments-retry.csv"`
	PriorityNodesPath string `help:"list of priority node ID-s" default:""`

	Check       int `help:"how many storagenodes to query per segment" default:"3"`
	BatchSize   int `help:"number of segments to process per batch" default:"10000"`
	Concurrency int `help:"number of concurrent verifiers" default:"1000"`

	AsOfSystemInterval time.Duration `help:"as of system interval" releaseDefault:"-5m" devDefault:"-1us" testDefault:"-1us"`
}

// Service implements segment verification logic.
type Service struct {
	log    *zap.Logger
	config ServiceConfig

	notFound SegmentWriter
	retry    SegmentWriter

	metabase Metabase
	verifier Verifier
	overlay  Overlay

	aliasToNodeURL map[metabase.NodeAlias]storj.NodeURL
	priorityNodes  NodeAliasSet
	onlineNodes    NodeAliasSet
}

// NewService returns a new service for verifying segments.
func NewService(log *zap.Logger, metabaseDB Metabase, verifier Verifier, overlay Overlay, config ServiceConfig) (*Service, error) {
	notFound, err := NewCSVWriter(config.NotFoundPath)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	retry, err := NewCSVWriter(config.RetryPath)
	if err != nil {
		return nil, errs.Combine(Error.Wrap(err), notFound.Close())
	}

	return &Service{
		log:    log,
		config: config,

		notFound: notFound,
		retry:    retry,

		metabase: metabaseDB,
		verifier: verifier,
		overlay:  overlay,

		aliasToNodeURL: map[metabase.NodeAlias]storj.NodeURL{},
		priorityNodes:  NodeAliasSet{},
		onlineNodes:    NodeAliasSet{},
	}, nil
}

// Close closes the outputs from the service.
func (service *Service) Close() error {
	return Error.Wrap(errs.Combine(
		service.notFound.Close(),
		service.retry.Close(),
	))
}

// loadOnlineNodes loads the list of online nodes.
func (service *Service) loadOnlineNodes(ctx context.Context) (err error) {
	interval := overlay.AsOfSystemTimeConfig{
		Enabled:         service.config.AsOfSystemInterval != 0,
		DefaultInterval: service.config.AsOfSystemInterval,
	}

	// should this use some other methods?
	nodes, err := service.overlay.SelectAllStorageNodesDownload(ctx, time.Hour, interval)
	if err != nil {
		return Error.Wrap(err)
	}

	nodeIDs := make([]storj.NodeID, len(nodes))
	for i, node := range nodes {
		nodeIDs[i] = node.ID
	}

	aliases, err := service.metabase.ConvertNodesToAliases(ctx, nodeIDs)
	if err != nil {
		return Error.Wrap(err)
	}

	for i, alias := range aliases {
		service.aliasToNodeURL[alias] = storj.NodeURL{
			ID:      nodes[i].ID,
			Address: nodes[i].Address.Address,
		}
		service.onlineNodes.Add(alias)
	}

	return nil
}

// loadPriorityNodes loads the list of priority nodes.
func (service *Service) loadPriorityNodes(ctx context.Context) (err error) {
	if service.config.PriorityNodesPath == "" {
		return nil
	}

	data, err := ioutil.ReadFile(service.config.PriorityNodesPath)
	if err != nil {
		return Error.New("unable to read priority nodes: %w", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		nodeID, err := storj.NodeIDFromString(line)
		if err != nil {
			return Error.Wrap(err)
		}

		aliases, err := service.metabase.ConvertNodesToAliases(ctx, []storj.NodeID{nodeID})
		if err != nil {
			service.log.Info("priority node ID not used", zap.Stringer("node id", nodeID), zap.Error(err))
			continue
		}

		service.priorityNodes.Add(aliases[0])
	}

	return nil
}

// ProcessRange processes segments between low and high uuid.UUID with the specified batchSize.
func (service *Service) ProcessRange(ctx context.Context, low, high uuid.UUID) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = service.loadOnlineNodes(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	err = service.loadPriorityNodes(ctx)
	if err != nil {
		return Error.Wrap(err)
	}

	cursorStreamID := low
	var cursorPosition metabase.SegmentPosition
	if !low.IsZero() {
		cursorStreamID = uuidBefore(low)
		cursorPosition = metabase.SegmentPosition{Part: 0xFFFFFFFF, Index: 0xFFFFFFFF}
	}

	for {
		result, err := service.metabase.ListVerifySegments(ctx, metabase.ListVerifySegments{
			CursorStreamID: cursorStreamID,
			CursorPosition: cursorPosition,
			Limit:          service.config.BatchSize,

			AsOfSystemInterval: service.config.AsOfSystemInterval,
		})
		if err != nil {
			return Error.Wrap(err)
		}
		verifySegments := result.Segments
		result.Segments = nil

		// drop any segment that's equal or beyond "high".
		for len(verifySegments) > 0 && !verifySegments[len(verifySegments)-1].StreamID.Less(high) {
			verifySegments = verifySegments[:len(verifySegments)-1]
		}

		// All done?
		if len(verifySegments) == 0 {
			return nil
		}

		last := &verifySegments[len(verifySegments)-1]
		cursorStreamID, cursorPosition = last.StreamID, last.Position

		// Convert to struct that contains the status.
		segmentsData := make([]Segment, len(verifySegments))
		segments := make([]*Segment, len(verifySegments))
		for i := range segments {
			segmentsData[i].VerifySegment = verifySegments[i]
			segments[i] = &segmentsData[i]
		}

		// Process the data.
		err = service.ProcessSegments(ctx, segments)
		if err != nil {
			return Error.Wrap(err)
		}
	}
}

// ProcessSegments processes a collection of segments.
func (service *Service) ProcessSegments(ctx context.Context, segments []*Segment) (err error) {
	defer mon.Task()(&ctx)(&err)

	service.log.Info("processing segments",
		zap.Int("count", len(segments)),
		zap.Stringer("first", segments[0].StreamID),
		zap.Stringer("last", segments[len(segments)-1].StreamID),
	)

	// Verify all the segments against storage nodes.
	err = service.Verify(ctx, segments)
	if err != nil {
		return Error.Wrap(err)
	}

	notFound := []*Segment{}
	retry := []*Segment{}

	// Find out which of the segments we did not find
	// or there was some other failure.
	for _, segment := range segments {
		if segment.Status.NotFound > 0 {
			notFound = append(notFound, segment)
		} else if segment.Status.Retry > 0 {
			// TODO: should we do a smarter check here?
			// e.g. if at least half did find, then consider it ok?
			retry = append(retry, segment)
		}
	}

	// Some segments might have been deleted during the
	// processing, so cross-reference and remove any deleted
	// segments from the list.
	notFound, err = service.RemoveDeleted(ctx, notFound)
	if err != nil {
		return Error.Wrap(err)
	}
	retry, err = service.RemoveDeleted(ctx, retry)
	if err != nil {
		return Error.Wrap(err)
	}

	// Output the problematic segments:
	errNotFound := service.notFound.Write(ctx, notFound)
	errRetry := service.retry.Write(ctx, retry)

	return errs.Combine(errNotFound, errRetry)
}

// RemoveDeleted modifies the slice and returns only the segments that
// still exist in the database.
func (service *Service) RemoveDeleted(ctx context.Context, segments []*Segment) (_ []*Segment, err error) {
	defer mon.Task()(&ctx)(&err)

	valid := segments[:0]
	for _, seg := range segments {
		_, err := service.metabase.GetSegmentByPosition(ctx, metabase.GetSegmentByPosition{
			StreamID: seg.StreamID,
			Position: seg.Position,
		})
		if metabase.ErrSegmentNotFound.Has(err) {
			continue
		}
		if err != nil {
			service.log.Error("get segment by id failed", zap.Stringer("stream-id", seg.StreamID), zap.String("position", fmt.Sprint(seg.Position)))
			if ctx.Err() != nil {
				return valid, ctx.Err()
			}
		}
		valid = append(valid, seg)
	}
	return valid, nil
}

// Segment contains minimal information necessary for verifying a single Segment.
type Segment struct {
	metabase.VerifySegment
	Status Status
}

// Status contains the statistics about the segment.
type Status struct {
	Retry    int32
	Found    int32
	NotFound int32
}

// MarkFound moves a retry token from retry to found.
func (status *Status) MarkFound() {
	atomic.AddInt32(&status.Retry, -1)
	atomic.AddInt32(&status.Found, 1)
}

// MarkNotFound moves a retry token from retry to not found.
func (status *Status) MarkNotFound() {
	atomic.AddInt32(&status.Retry, -1)
	atomic.AddInt32(&status.NotFound, 1)
}

// Batch is a list of segments to be verified on a single node.
type Batch struct {
	Alias metabase.NodeAlias
	Items []*Segment
}

// Len returns the length of the batch.
func (b *Batch) Len() int { return len(b.Items) }

// uuidBefore returns an uuid.UUID that's immediately before v.
// It might not be a valid uuid after this operation.
func uuidBefore(v uuid.UUID) uuid.UUID {
	for i := len(v) - 1; i >= 0; i-- {
		v[i]--
		if v[i] != 0xFF { // we didn't wrap around
			break
		}
	}
	return v
}
