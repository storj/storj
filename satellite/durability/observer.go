// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package durability

import (
	"context"
	"time"

	"github.com/jtolio/eventkit"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/metabase/rangedloop"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

var ek = eventkit.Package()

// HealthStat collects the availability conditions for one class (for example: nodes with the same owner).
type HealthStat struct {
	// because 0 means uninitialized, we store the min +1
	minPlusOne int
}

// Update updates the stat with one measurement: number of pieces which are available even without the nodes of the selected class.
func (h *HealthStat) Update(num int) {
	if num < h.minPlusOne-1 || h.minPlusOne == 0 {
		h.minPlusOne = num + 1
	}
}

// Merge can merge two stat to one, without losing information.
func (h *HealthStat) Merge(stat *HealthStat) {
	if stat.minPlusOne < h.minPlusOne && stat.minPlusOne > 0 {
		h.minPlusOne = stat.minPlusOne
	}
}

// Min returns the minimal number.
func (h *HealthStat) Min() int {
	return h.minPlusOne - 1
}

// Unused returns true when stat is uninitialized (-1) and was not updated with any number.
func (h *HealthStat) Unused() bool {
	return h.minPlusOne == 0
}

// NodeClassifier identifies a risk class (for example an owner, or country) of the SelectedNode.
type NodeClassifier func(node *nodeselection.SelectedNode) string

// ReportConfig configures durability report.
type ReportConfig struct {
	Enabled bool `help:"whether to enable durability report (rangedloop observer)" default:"true"`
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
	healthStat         map[string]*HealthStat
	classifiers        []NodeClassifier
	aliasMap           *metabase.NodeAliasMap
	nodes              map[storj.NodeID]*nodeselection.SelectedNode
	db                 overlay.DB
	metabaseDB         *metabase.DB
	reporter           func(name string, stat *HealthStat)
	reportThreshold    int
	asOfSystemInterval time.Duration
}

// NewDurability creates the new instance.
func NewDurability(db overlay.DB, metabaseDB *metabase.DB, classifiers []NodeClassifier, reportThreshold int, asOfSystemInterval time.Duration) *Report {
	return &Report{
		db:                 db,
		metabaseDB:         metabaseDB,
		classifiers:        classifiers,
		reportThreshold:    reportThreshold,
		asOfSystemInterval: asOfSystemInterval,
		nodes:              make(map[storj.NodeID]*nodeselection.SelectedNode),
		healthStat:         make(map[string]*HealthStat),
		reporter:           reportToEventkit,
	}
}

// Start implements rangedloop.Observer.
func (c *Report) Start(ctx context.Context, startTime time.Time) error {
	nodes, err := c.db.GetParticipatingNodes(ctx, -12*time.Hour, c.asOfSystemInterval)
	if err != nil {
		return errs.Wrap(err)
	}
	c.nodes = map[storj.NodeID]*nodeselection.SelectedNode{}
	for ix := range nodes {
		c.nodes[nodes[ix].ID] = &nodes[ix]
	}
	aliasMap, err := c.metabaseDB.LatestNodesAliasMap(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	c.aliasMap = aliasMap
	return nil
}

// Fork implements rangedloop.Observer.
func (c *Report) Fork(ctx context.Context) (rangedloop.Partial, error) {
	d := &ObserverFork{
		classifiers:     c.classifiers,
		healthStat:      nil,
		aliasMap:        c.aliasMap,
		nodes:           c.nodes,
		classifierCache: make([][]string, c.aliasMap.Max()+1),
		reportThreshold: c.reportThreshold,
	}
	d.classifyNodeAliases()
	return d, nil
}

// Join implements rangedloop.Observer.
func (c *Report) Join(ctx context.Context, partial rangedloop.Partial) (err error) {
	defer mon.Task()(&ctx)(&err)
	fork := partial.(*ObserverFork)
	for cid, stat := range fork.healthStat {
		if stat.Unused() {
			continue
		}
		name := fork.className[classID(cid)]
		existing, found := c.healthStat[name]
		if !found {
			c.healthStat[name] = &HealthStat{
				minPlusOne: stat.minPlusOne,
			}
		} else {
			existing.Merge(&stat)
		}

	}
	return nil
}

// Finish implements rangedloop.Observer.
func (c *Report) Finish(ctx context.Context) error {
	for name, stat := range c.healthStat {
		c.reporter(name, stat)
	}
	return nil
}

// TestChangeReporter modifies the reporter for unit tests.
func (c *Report) TestChangeReporter(r func(name string, stat *HealthStat)) {
	c.reporter = r
}

// classID is a fork level short identifier for each class.
type classID int32

// ObserverFork is the durability calculator for each segment range.
type ObserverFork struct {
	// map between classes (like "country:hu" and integer IDs)
	classID   map[string]classID
	className map[classID]string

	controlledByClassCache []int32

	healthStat      []HealthStat
	classifiers     []NodeClassifier
	aliasMap        *metabase.NodeAliasMap
	nodes           map[storj.NodeID]*nodeselection.SelectedNode
	classifierCache [][]string

	// contains the available classes for each node alias.
	classified      [][]classID
	reportThreshold int
}

func (c *ObserverFork) classifyNodeAliases() {
	c.classID = make(map[string]classID, len(c.classifiers))
	c.className = make(map[classID]string, len(c.classifiers))

	c.classified = make([][]classID, c.aliasMap.Max()+1)
	for _, node := range c.nodes {
		alias, ok := c.aliasMap.Alias(node.ID)
		if !ok {
			continue
		}

		classes := make([]classID, len(c.classifiers))
		for i, group := range c.classifiers {
			class := group(node)
			id, ok := c.classID[class]
			if !ok {
				id = classID(len(c.classID))
				c.className[id] = class
				c.classID[class] = id
			}
			classes[i] = id
		}
		c.classified[alias] = classes
	}
	c.healthStat = make([]HealthStat, len(c.classID))
	c.controlledByClassCache = make([]int32, len(c.classID))
}

// Process implements rangedloop.Partial.
func (c *ObserverFork) Process(ctx context.Context, segments []rangedloop.Segment) (err error) {
	controlledByClass := c.controlledByClassCache
	for i := range segments {
		s := &segments[i]
		healthyPieceCount := 0
		for _, piece := range s.AliasPieces {
			classes := c.classified[piece.Alias]

			// unavailable/offline nodes were not classified
			if len(classes) > 0 {
				healthyPieceCount++
			}

			for _, class := range classes {
				controlledByClass[class]++
			}
		}

		for classID, count := range controlledByClass {
			if count == 0 {
				continue
			}

			// reset the value for the next iteration
			controlledByClass[classID] = 0

			diff := healthyPieceCount - int(count)

			// if value is high, it's not a problem. faster to ignore it...
			if c.reportThreshold > 0 && diff > c.reportThreshold {
				continue
			}
			c.healthStat[classID].Update(diff)
		}
	}
	return nil
}

func reportToEventkit(name string, stat *HealthStat) {
	ek.Event("durability", eventkit.String("name", name), eventkit.Int64("min", int64(stat.Min())))
}

var _ rangedloop.Observer = &Report{}
var _ rangedloop.Partial = &ObserverFork{}
