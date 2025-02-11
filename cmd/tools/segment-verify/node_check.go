// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/process"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/satellitedb"
)

func verifySegmentsNodeCheck(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)
	log := zap.L()

	// open default satellite database
	db, err := satellitedb.Open(ctx, log.Named("db"), satelliteCfg.Database, satellitedb.Options{
		ApplicationName:     "segment-verify",
		SaveRollupBatchSize: satelliteCfg.Tally.SaveRollupBatchSize,
		ReadRollupBatchSize: satelliteCfg.Tally.ReadRollupBatchSize,
	})
	if err != nil {
		return errs.New("Error starting master database on satellite: %+v", err)
	}
	defer func() {
		err = errs.Combine(err, db.Close())
	}()

	// open metabase
	metabaseDB, err := metabase.Open(ctx, log.Named("metabase"), satelliteCfg.Metainfo.DatabaseURL,
		satelliteCfg.Config.Metainfo.Metabase("satellite-core"))
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { _ = metabaseDB.Close() }()

	// check whether satellite and metabase versions match
	versionErr := db.CheckVersion(ctx)
	if versionErr != nil {
		log.Error("versions skewed", zap.Error(versionErr))
		return Error.Wrap(versionErr)
	}

	versionErr = metabaseDB.CheckVersion(ctx)
	if versionErr != nil {
		log.Error("versions skewed", zap.Error(versionErr))
		return Error.Wrap(versionErr)
	}

	service, err := NewNodeCheckService(log, metabaseDB, db.OverlayCache(), nodeCheckCfg)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() { err = errs.Combine(err, service.Close()) }()

	return service.ProcessAll(ctx)
}

// NodeCheckConfig defines configuration for verifying segment existence.
type NodeCheckConfig struct {
	BatchSize       int  `help:"number of segments to process per batch" default:"10000"`
	DuplicatesLimit int  `help:"maximum duplicates allowed" default:"3"`
	UnvettedLimit   int  `help:"maximum unvetted allowed" default:"9"`
	IncludeAllNodes bool `help:"include disqualified and exited nodes in the node check" default:"false"`

	AsOfSystemInterval time.Duration `help:"as of system interval" releaseDefault:"-5m" devDefault:"-1us" testDefault:"-1us"`
}

// NodeCheckOverlayDB contains dependencies from overlay that are needed for the processing.
type NodeCheckOverlayDB interface {
	IterateAllContactedNodes(context.Context, func(context.Context, *nodeselection.SelectedNode) error) error
	IterateAllNodeDossiers(context.Context, func(context.Context, *overlay.NodeDossier) error) error
}

// NodeCheckService implements a service for checking duplicate nets being used in a segment.
type NodeCheckService struct {
	log    *zap.Logger
	config NodeCheckConfig

	metabase Metabase
	overlay  NodeCheckOverlayDB

	// lookup tables for nodes
	aliasMap *metabase.NodeAliasMap

	// alias table for lastNet lookups
	netAlias           map[string]netAlias
	nodeInfoByNetAlias map[metabase.NodeAlias]nodeInfo

	// table for converting a node alias to a net alias
	nodeAliasToNetAlias []netAlias

	scratch bitset
}

// netAlias represents a unique ID for a given subnet.
type netAlias int

type nodeInfo struct {
	vetted       bool
	disqualified bool
	exited       bool
}

// NewNodeCheckService returns a new service for verifying segments.
func NewNodeCheckService(log *zap.Logger, metabaseDB Metabase, overlay NodeCheckOverlayDB, config NodeCheckConfig) (*NodeCheckService, error) {
	return &NodeCheckService{
		log:    log,
		config: config,

		metabase: metabaseDB,
		overlay:  overlay,
	}, nil
}

// Close closes the service.
func (service *NodeCheckService) Close() (err error) {
	return nil
}

// init sets up tables for quick verification.
func (service *NodeCheckService) init(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	service.aliasMap, err = service.metabase.LatestNodesAliasMap(ctx)
	if err != nil {
		return Error.Wrap(err)
	}
	service.netAlias = make(map[string]netAlias)
	service.nodeInfoByNetAlias = make(map[metabase.NodeAlias]nodeInfo)
	service.nodeAliasToNetAlias = make([]netAlias, service.aliasMap.Max()+1)

	err = service.overlay.IterateAllNodeDossiers(ctx, func(ctx context.Context, node *overlay.NodeDossier) error {
		nodeAlias, ok := service.aliasMap.Alias(node.Id)
		if !ok {
			// some nodes aren't in the metabase
			return nil
		}

		// assign unique ID-s for all nets
		net := node.LastNet
		alias, ok := service.netAlias[net]
		if !ok {
			alias = netAlias(len(service.netAlias))
			service.netAlias[net] = alias
		}
		nodeInfo := nodeInfo{
			vetted:       node.Reputation.Status.VettedAt != nil,
			disqualified: node.Reputation.Status.Disqualified != nil,
			exited:       node.ExitStatus.ExitFinishedAt != nil,
		}
		service.nodeInfoByNetAlias[nodeAlias] = nodeInfo
		service.nodeAliasToNetAlias[nodeAlias] = alias
		return nil
	})
	if err != nil {
		return Error.Wrap(err)
	}

	service.scratch = newBitSet(len(service.netAlias))

	return nil
}

// ProcessAll processes all segments with the specified batchSize.
func (service *NodeCheckService) ProcessAll(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	if err := service.init(ctx); err != nil {
		return Error.Wrap(err)
	}

	var cursorStreamID uuid.UUID
	var cursorPosition metabase.SegmentPosition

	var progress int64
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
		segments := result.Segments

		// All done?
		if len(segments) == 0 {
			return nil
		}

		last := &segments[len(segments)-1]
		cursorStreamID, cursorPosition = last.StreamID, last.Position

		service.log.Info("processing segments",
			zap.Int64("progress", progress),
			zap.Int("count", len(segments)),
			zap.Stringer("first", segments[0].StreamID),
			zap.Stringer("last", segments[len(segments)-1].StreamID),
		)
		progress += int64(len(segments))

		// Process the data.
		for _, segment := range segments {
			if err := service.Verify(ctx, segment); err != nil {
				service.log.Warn("found",
					zap.Stringer("stream-id", segment.StreamID),
					zap.Uint64("position", segment.Position.Encode()),
					zap.Error(err))
			}
		}
	}
}

// Verify verifies a single segment.
func (service *NodeCheckService) Verify(ctx context.Context, segment metabase.VerifySegment) (err error) {
	// intentionally no monitoring for performance
	scratch := service.scratch
	scratch.Clear()

	count := 0
	unvetted := 0
	for _, alias := range segment.AliasPieces {
		if alias.Alias >= metabase.NodeAlias(len(service.nodeAliasToNetAlias)) {
			continue
		}

		nodeInfo := service.nodeInfoByNetAlias[alias.Alias]
		if !service.config.IncludeAllNodes &&
			(nodeInfo.disqualified || nodeInfo.exited) {
			continue
		}

		if !nodeInfo.vetted {
			unvetted++
		}

		netAlias := service.nodeAliasToNetAlias[alias.Alias]
		if scratch.Include(int(netAlias)) {
			count++
		}
	}
	if count > service.config.DuplicatesLimit || unvetted > service.config.UnvettedLimit {
		fmt.Printf("%s\t%d\t%d\t%d\t%v\t%v\n", segment.StreamID, segment.Position.Encode(), count, unvetted, segment.CreatedAt, segment.RepairedAt)
	}

	return nil
}

type bitset []uint32

func newBitSet(size int) bitset {
	return bitset(make([]uint32, (size+31)/32))
}

func (set bitset) offset(index int) (bucket, bitmask uint32) {
	return uint32(index / 32), uint32(1 << (index % 32))
}

func (set bitset) Include(index int) bool {
	bucket, bitmask := set.offset(index)
	had := set[bucket]&bitmask != 0
	set[bucket] |= bitmask
	return had
}

func (set bitset) Clear() {
	for i := range set {
		set[i] = 0
	}
}
