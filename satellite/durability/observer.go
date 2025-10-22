// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package durability

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/eventkit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/satellite/repair"
)

var ek = eventkit.Package()

const maxExemplars = 3

// HistogramByPlacement contains multiple Histogram for each Placement.
type HistogramByPlacement []Histogram // placement -> Histogram

// HealthStat stores the number of segments for each piece count (healthy / retrievable pairs).
type HealthStat struct {
	Placement         storj.PlacementConstraint
	HealthyPieces     int // k-normalized number of healthy pieces
	RetrievablePieces int // k-normalized number of retrievable pieces
	SegmentCount      int
	Exemplar          string
}

// HealthMatrix stores the number of segments for each k+healthy / k+ retrievable pieces.
type HealthMatrix struct {
	Stat []HealthStat
}

// Increment updates the health matrix counter for the given parameters.
func (h *HealthMatrix) Increment(placement storj.PlacementConstraint, healthyPieces int, retrievablePieces int, increment int, segmentExemplar string) {
	for ix := range h.Stat {
		if h.Stat[ix].Placement == placement && h.Stat[ix].HealthyPieces == healthyPieces && h.Stat[ix].RetrievablePieces == retrievablePieces {
			h.Stat[ix].SegmentCount += increment
			return
		}
	}
	h.Stat = append(h.Stat, HealthStat{
		Placement:         placement,
		HealthyPieces:     healthyPieces,
		RetrievablePieces: retrievablePieces,
		Exemplar:          segmentExemplar,
		SegmentCount:      increment,
	})
}

// Merge merges the health matrix with another health matrix.
func (h *HealthMatrix) Merge(matrix *HealthMatrix) {
	for _, stat := range matrix.Stat {
		h.Increment(stat.Placement, stat.HealthyPieces, stat.RetrievablePieces, stat.SegmentCount, stat.Exemplar)
	}
}

// Find returns the number of segments for the given placement, healthyPieces and retrievablePieces.
func (h *HealthMatrix) Find(placement storj.PlacementConstraint, healthyPieces, retrievablePieces int) int {
	for _, stat := range h.Stat {
		if stat.Placement == placement && stat.HealthyPieces == healthyPieces && stat.RetrievablePieces == retrievablePieces {
			return stat.SegmentCount
		}
	}
	return 0
}

// Reset will reset all the included Histograms.
func (p *HistogramByPlacement) Reset() {
	for _, h := range *p {
		h.Reset()
	}
}

// Extend will make sure the placement has an initialized Histogram.
func (p *HistogramByPlacement) Extend(size int) {
	if len(*p) <= size {
		*p = append(*p, make([]Histogram, size-len(*p)+1)...)
	}
}

// Histogram stores the number of segments (and some exemplars) for each piece count.
type Histogram struct {
	// pieceCount -> {number of segments, exemplars}
	Buckets []*Bucket
	// pieceCount * -1 -> {number of segments, exemplars}
	NegativeBuckets []*Bucket
}

// Bucket stores the number of segments (and some exemplars) for each piece count.
type Bucket struct {
	SegmentCount     int
	SegmentExemplars []string
	ClassExemplars   []ClassID
}

// NodeGetter returns node information required by the durability loop.
type NodeGetter interface {
	GetNodes(ctx context.Context, validUpTo time.Time, nodeIDs []storj.NodeID, selectedNodes []nodeselection.SelectedNode) ([]nodeselection.SelectedNode, error)
}

// Increment increments the bucket counters.
func (b *Bucket) Increment(segmentExemplar uuid.UUID, segmentExemplarPosition metabase.SegmentPosition, classExemplar ClassID) {
	b.SegmentCount++
	if len(b.SegmentExemplars) < maxExemplars {
		b.SegmentExemplars = append(b.SegmentExemplars, fmt.Sprintf("%s/%d", segmentExemplar, segmentExemplarPosition.Encode()))
	}
	if len(b.ClassExemplars) < maxExemplars {
		b.ClassExemplars = append(b.ClassExemplars, classExemplar)
	}
}

// Reset resets the bucket counters.
func (b *Bucket) Reset() {
	b.SegmentCount = 0
	b.SegmentExemplars = b.SegmentExemplars[:0]
	b.ClassExemplars = b.ClassExemplars[:0]
}

// AddPieceCount adds a piece count to the histogram.
func (h *Histogram) AddPieceCount(pieceCount int, segmentExemplar uuid.UUID, segmentExemplarPosition metabase.SegmentPosition, classExemplar ClassID) {
	if pieceCount < 0 {
		for len(h.NegativeBuckets) <= -pieceCount {
			h.NegativeBuckets = append(h.NegativeBuckets, &Bucket{})
		}
		h.NegativeBuckets[-pieceCount].Increment(segmentExemplar, segmentExemplarPosition, classExemplar)
	} else {
		for len(h.Buckets) <= pieceCount {
			h.Buckets = append(h.Buckets, &Bucket{})
		}
		h.Buckets[pieceCount].Increment(segmentExemplar, segmentExemplarPosition, classExemplar)
	}
}

// Reset resets the histogram counters.
func (h *Histogram) Reset() {
	for _, b := range h.Buckets {
		b.Reset()
	}
	for _, b := range h.NegativeBuckets {
		b.Reset()
	}
}

// Merge merges the histogram with another histogram.
func (h *Histogram) Merge(o Histogram) {
	for i, b := range o.Buckets {
		if len(h.Buckets) <= i {
			h.Buckets = append(h.Buckets, &Bucket{})
		}
		h.Buckets[i].SegmentCount += b.SegmentCount
		h.Buckets[i].SegmentExemplars = append(h.Buckets[i].SegmentExemplars, b.SegmentExemplars...)
		h.Buckets[i].ClassExemplars = append(h.Buckets[i].ClassExemplars, b.ClassExemplars...)
	}
	for i, b := range o.NegativeBuckets {
		if len(h.NegativeBuckets) <= i {
			h.NegativeBuckets = append(h.NegativeBuckets, &Bucket{})
		}
		h.NegativeBuckets[i].SegmentCount += b.SegmentCount
		if len(h.NegativeBuckets[i].SegmentExemplars) < maxExemplars {
			h.NegativeBuckets[i].SegmentExemplars = append(h.NegativeBuckets[i].SegmentExemplars, b.SegmentExemplars...)
		}
		if len(h.NegativeBuckets[i].ClassExemplars) < maxExemplars {
			h.NegativeBuckets[i].ClassExemplars = append(h.NegativeBuckets[i].ClassExemplars, b.ClassExemplars...)
		}
	}
}

// NodeClassifier identifies a risk class (for example an owner, or country) of the SelectedNode.
type NodeClassifier func(node *nodeselection.SelectedNode) string

// ReportConfig configures durability report.
type ReportConfig struct {
	Enabled bool `help:"whether to enable durability report (rangedloop observer)" default:"true" testDefault:"false"`
}

// Report  is a calculator (rangloop.Observer) which checks the availability of pieces without certain nodes.
// It can answer the following question:
//  1. loosing a given group of nodes (all nodes of one country or all nodes of one owner)...
//  2. what will be the lowest humber of healhty pieces, checking all the segments.
//
// Example: we have one segment where 80 pieces are stored, but 42 of them are in Germany.
//
//	in this case this reporter will return 38 for the class "country:DE" (assuming all the other segments are more lucky).
type Report struct {
	// histogram shows the piece distribution -> by default, 1 -> with removing the biggest owner from each piece, 2-> with removing the second biggest owner from each piece, etc.
	healthStat []HistogramByPlacement
	// healthMatrix stores the number of segments for each k+healthy / k+ retrievable pieces.
	healthMatrix *HealthMatrix

	classifier         NodeClassifier
	nodes              []nodeselection.SelectedNode
	db                 overlay.DB
	metabaseDB         *metabase.DB
	reporter           func(n time.Time, class string, missingProviders int, ix int, placement storj.PlacementConstraint, stat Bucket, classResolver func(id ClassID) string)
	matrixReporter     func(n time.Time, placement storj.PlacementConstraint, healthy int, retrievable int, segmentCount int, segmentExemplar string)
	asOfSystemInterval time.Duration

	// map between classes (like "country:hu" and integer IDs)
	className map[ClassID]string

	// contains the available classes for each node alias.
	classified []ClassID

	Class      string
	nodeGetter NodeGetter
}

// NewDurability creates the new instance.
func NewDurability(db overlay.DB, metabaseDB *metabase.DB, nodeGetter NodeGetter, class string, classifier NodeClassifier, asOfSystemInterval time.Duration) *Report {
	return &Report{
		healthStat:         make([]HistogramByPlacement, 3),
		healthMatrix:       &HealthMatrix{},
		nodeGetter:         nodeGetter,
		Class:              class,
		db:                 db,
		metabaseDB:         metabaseDB,
		classifier:         classifier,
		asOfSystemInterval: asOfSystemInterval,
		nodes:              []nodeselection.SelectedNode{},
		reporter:           reportToEventkit,
		matrixReporter:     matrixReportToEventkit,
	}
}

// Start implements rangedloop.Observer.
func (c *Report) Start(ctx context.Context, startTime time.Time) (err error) {
	c.nodes, err = c.db.GetAllParticipatingNodes(ctx, -12*time.Hour, c.asOfSystemInterval)
	if err != nil {
		return errs.Wrap(err)
	}
	aliasMap, err := c.metabaseDB.LatestNodesAliasMap(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	c.resetStat()
	c.classifyNodeAliases(aliasMap)
	return nil
}

func (c *Report) resetStat() {
	for _, h := range c.healthStat {
		h.Reset()
	}
	c.healthMatrix = &HealthMatrix{}
}

func (c *Report) classifyNodeAliases(aliasMap *metabase.NodeAliasMap) {
	classes := make(map[string]ClassID)
	c.className = make(map[ClassID]string)

	classes["unclassified"] = 0
	c.className[0] = "unclassified"

	c.classified = make([]ClassID, aliasMap.Max()+1)
	for _, node := range c.nodes {
		alias, ok := aliasMap.Alias(node.ID)
		if !ok {
			continue
		}

		class := c.classifier(&node)
		id, ok := classes[class]
		if !ok {
			id = ClassID(len(classes))
			c.className[id] = class
			classes[class] = id
		}
		c.classified[alias] = id
	}
}

// Fork implements rangedloop.Observer.
func (c *Report) Fork(ctx context.Context) (rangedloop.Partial, error) {
	d := &ObserverFork{
		nodesCache:             c.nodeGetter,
		healthStat:             make([]HistogramByPlacement, 3),
		controlledByClassCache: make([]int32, len(c.className)),
		classified:             c.classified,
		healthMatrix:           &HealthMatrix{},
	}
	return d, nil
}

// Join implements rangedloop.Observer.
func (c *Report) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	defer mon.Task()(&ctx)(&err)
	fork := partial.(*ObserverFork)
	for ix := range fork.healthStat {
		if len(c.healthStat) <= ix {
			c.healthStat = append(c.healthStat, HistogramByPlacement{})
		}
		for placement, hs := range fork.healthStat[ix] {
			c.healthStat[ix].Extend(placement)
			c.healthStat[ix][placement].Merge(hs)
		}
	}
	c.healthMatrix.Merge(fork.healthMatrix)
	return nil
}

func (c *Report) resolveClass(id ClassID) string {
	return c.className[id]
}

// Finish implements rangedloop.Observer.
func (c *Report) Finish(ctx context.Context) error {
	reportTime := time.Now()
	for group, healthStat := range c.healthStat {
		for placement, histogram := range healthStat {
			for ix, stat := range histogram.Buckets {
				c.reporter(reportTime, c.Class, group, ix, storj.PlacementConstraint(placement), *stat, c.resolveClass)
			}
			for ix, stat := range histogram.NegativeBuckets {
				c.reporter(reportTime, c.Class, group, ix*-1, storj.PlacementConstraint(placement), *stat, c.resolveClass)
			}
		}
	}
	for _, stat := range c.healthMatrix.Stat {
		c.matrixReporter(reportTime, stat.Placement, stat.HealthyPieces, stat.RetrievablePieces, stat.SegmentCount, stat.Exemplar)
	}
	return nil
}

// TestChangeReporter modifies the reporter for unit tests.
func (c *Report) TestChangeReporter(r func(n time.Time, class string, missingProvider int, ix int, placement storj.PlacementConstraint, stat Bucket, resolver func(id ClassID) string)) {
	c.reporter = r
}

// GetClass return with the class instance name (like last_net or country).
func (c *Report) GetClass() string {
	return c.Class
}

// ClassID is a fork level short identifier for each class.
type ClassID int32

// ObserverFork is the durability calculator for each segment range.
type ObserverFork struct {
	controlledByClassCache []int32
	healthStat             []HistogramByPlacement
	healthMatrix           *HealthMatrix

	nodesCache NodeGetter

	// reuse those slices to optimize memory usage
	nodeIDs []storj.NodeID
	nodes   []nodeselection.SelectedNode

	classified []ClassID
	placements nodeselection.PlacementDefinitions
}

// Process implements rangedloop.Partial.
func (c *ObserverFork) Process(ctx context.Context, segments []rangedloop.Segment) (err error) {

	for i := range segments {
		s := &segments[i]

		if s.Inline() {
			continue
		}
		// ignore segment if expired
		if s.Expired(time.Now()) {
			continue
		}

		pieces := s.Pieces
		if len(pieces) == 0 {
			continue
		}

		// reuse fork.nodeIDs and fork.nodes slices if large enough
		if cap(c.nodeIDs) < len(pieces) {
			c.nodeIDs = make([]storj.NodeID, len(pieces))
			c.nodes = make([]nodeselection.SelectedNode, len(pieces))
		} else {
			c.nodeIDs = c.nodeIDs[:len(pieces)]
			c.nodes = c.nodes[:len(pieces)]
		}

		for i, piece := range pieces {
			c.nodeIDs[i] = piece.StorageNode
		}

		selectedNodes, err := c.nodesCache.GetNodes(ctx, s.CreatedAt, c.nodeIDs, c.nodes)
		if err != nil {
			continue
		}

		piecesCheck := repair.ClassifySegmentPieces(s.Pieces, selectedNodes, nil, true, true, c.placements[s.Placement])

		c.healthMatrix.Increment(s.Placement, piecesCheck.Healthy.Count()-int(s.Redundancy.RequiredShares), piecesCheck.Retrievable.Count()-int(s.Redundancy.RequiredShares), 1, fmt.Sprintf("%s/%d", s.StreamID, s.Position.Encode()))
		counters := ClassGroupCounters{}
		healthyPieceCount := 0
		for _, piece := range s.AliasPieces {
			if len(c.classified) <= int(piece.Alias) {
				// this is a new node, but we can ignore it.
				// will be included in the next execution cycle.
				continue
			}

			if !piecesCheck.Healthy.Contains(int(piece.Number)) {
				continue
			}

			class := c.classified[piece.Alias]

			// unavailable/offline nodes were not classified
			if class == 0 {
				continue
			}
			counters.Increment(class)
			healthyPieceCount++
		}

		// calculate normal distribution + distribution without the biggest class(es)
		healthyPiceces := healthyPieceCount
		for i := 0; i < len(c.healthStat); i++ {
			var maxClass ClassID
			maxCount := 0
			maxIndex := -1
			for ix, counter := range counters.Counters {
				if counter.Counter > maxCount {
					maxCount = counter.Counter
					maxClass = counter.Class
					maxIndex = ix
				}
			}
			if maxIndex == -1 {
				break
			}
			c.healthStat[i].Extend(int(s.Placement))
			c.healthStat[i][s.Placement].AddPieceCount(healthyPiceces-int(s.Redundancy.RequiredShares), s.StreamID, s.Position, maxClass)
			healthyPiceces -= counters.Counters[maxIndex].Counter
			counters.Counters[maxIndex].Counter = 0
		}

	}
	return nil
}

func reportToEventkit(n time.Time, class string, missingProviders int, ix int, placement storj.PlacementConstraint, stat Bucket, classResolver func(id ClassID) string) {
	var classExemplars []string
	for _, ex := range stat.ClassExemplars {
		classExemplars = append(classExemplars, classResolver(ex))
	}
	ek.Event("durability",
		eventkit.String("class", class),
		eventkit.Int64("healthy_pieces", int64(ix)),
		eventkit.Int64("count", int64(stat.SegmentCount)),
		eventkit.Int64("missing_providers", int64(missingProviders)),
		eventkit.Int64("placement", int64(placement)),
		eventkit.String("segment_exemplars", strings.Join(stat.SegmentExemplars, ",")),
		eventkit.String("class_exemplars", strings.Join(classExemplars, ",")),
		eventkit.Timestamp("report_time", n),
	)
}

func matrixReportToEventkit(n time.Time, placement storj.PlacementConstraint, healthy int, retrievable int, count int, exemplar string) {
	ek.Event("durability_matrix",
		eventkit.Int64("placement", int64(placement)),
		eventkit.Int64("healthy_pieces", int64(healthy)),
		eventkit.Int64("retrievable_pieces", int64(retrievable)),
		eventkit.Int64("count", int64(count)),
		eventkit.String("segment_exemplars", exemplar),
		eventkit.Timestamp("report_time", n),
	)
}

var _ rangedloop.Observer = &Report{}

var _ rangedloop.Partial = &ObserverFork{}

// ClassGroupCounters is a helper struct to count the number of pieces in each class.
type ClassGroupCounters struct {
	Counters []ClassGroupCounter
}

// Increment increments the counter for the given class.
func (c *ClassGroupCounters) Increment(class ClassID) {
	for ix, counter := range c.Counters {
		if counter.Class == class {
			c.Counters[ix].Counter++
			return
		}
	}
	c.Counters = append(c.Counters, ClassGroupCounter{Class: class, Counter: 1})
}

// ClassGroupCounter is a helper struct to count the number of pieces in a class.
type ClassGroupCounter struct {
	Class   ClassID
	Counter int
}
