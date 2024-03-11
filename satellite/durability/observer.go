// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package durability

import (
	"context"
	"strconv"
	"time"

	"github.com/zeebo/errs"
	"golang.org/x/exp/slices"

	"storj.io/eventkit"
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
	Exemplar   string
}

// Update updates the stat with one measurement: number of pieces which are available even without the nodes of the selected class.
// Exemplar is one example identifier with such measurement. Useful to dig deeper, based on this one example.
func (h *HealthStat) Update(num int, exemplar string) {
	if num < h.minPlusOne-1 || h.minPlusOne == 0 {
		h.minPlusOne = num + 1
		h.Exemplar = exemplar
	}
}

// Merge can merge two stat to one, without losing information.
func (h *HealthStat) Merge(stat *HealthStat) {
	if stat.minPlusOne < h.minPlusOne && stat.minPlusOne > 0 {
		h.minPlusOne = stat.minPlusOne
		h.Exemplar = stat.Exemplar
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
	busFactor          HealthStat
	classifier         NodeClassifier
	aliasMap           *metabase.NodeAliasMap
	nodes              []nodeselection.SelectedNode
	db                 overlay.DB
	metabaseDB         *metabase.DB
	reporter           func(n time.Time, class string, value string, stat *HealthStat)
	reportThreshold    int
	busFactorThreshold int
	asOfSystemInterval time.Duration

	// map between classes (like "country:hu" and integer IDs)
	className map[classID]string

	// contains the available classes for each node alias.
	classified []classID

	maxPieceCount int
	class         string
}

// NewDurability creates the new instance.
func NewDurability(db overlay.DB, metabaseDB *metabase.DB, class string, classifier NodeClassifier, maxPieceCount int, reportThreshold int, busFactorThreshold int, asOfSystemInterval time.Duration) *Report {
	return &Report{
		class:              class,
		db:                 db,
		metabaseDB:         metabaseDB,
		classifier:         classifier,
		reportThreshold:    reportThreshold,
		busFactorThreshold: busFactorThreshold,
		asOfSystemInterval: asOfSystemInterval,
		healthStat:         make(map[string]*HealthStat),
		nodes:              []nodeselection.SelectedNode{},
		reporter:           reportToEventkit,
		maxPieceCount:      maxPieceCount,
	}
}

// Start implements rangedloop.Observer.
func (c *Report) Start(ctx context.Context, startTime time.Time) (err error) {
	c.nodes, err = c.db.GetParticipatingNodes(ctx, -12*time.Hour, c.asOfSystemInterval)
	if err != nil {
		return errs.Wrap(err)
	}
	c.aliasMap, err = c.metabaseDB.LatestNodesAliasMap(ctx)
	if err != nil {
		return errs.Wrap(err)
	}
	c.resetStat()
	c.classifyNodeAliases()
	return nil
}

func (c *Report) resetStat() {
	c.healthStat = make(map[string]*HealthStat)
	c.busFactor = HealthStat{}
}

func (c *Report) classifyNodeAliases() {
	classes := make(map[string]classID)
	c.className = make(map[classID]string)

	classes["unclassified"] = 0
	c.className[0] = "unclassified"

	c.classified = make([]classID, c.aliasMap.Max()+1)
	for _, node := range c.nodes {
		alias, ok := c.aliasMap.Alias(node.ID)
		if !ok {
			continue
		}

		class := c.classifier(&node)
		id, ok := classes[class]
		if !ok {
			id = classID(len(classes))
			c.className[id] = class
			classes[class] = id
		}
		c.classified[alias] = id
	}
}

// Fork implements rangedloop.Observer.
func (c *Report) Fork(ctx context.Context) (rangedloop.Partial, error) {
	d := &ObserverFork{
		reportThreshold:        c.reportThreshold,
		healthStat:             make([]HealthStat, len(c.className)),
		controlledByClassCache: make([]int32, len(c.className)),
		busFactorCache:         make([]int32, 0, c.maxPieceCount),
		classified:             c.classified,
	}
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
		name := c.className[classID(cid)]
		existing, found := c.healthStat[name]
		if !found {
			c.healthStat[name] = &HealthStat{
				minPlusOne: stat.minPlusOne,
				Exemplar:   stat.Exemplar,
			}
		} else {
			existing.Merge(&stat)
		}

	}
	c.busFactor.Update(fork.busFactor.Min(), fork.busFactor.Exemplar)
	return nil
}

// Finish implements rangedloop.Observer.
func (c *Report) Finish(ctx context.Context) error {
	reportTime := time.Now()
	for name, stat := range c.healthStat {
		c.reporter(reportTime, c.class, name, stat)
	}
	c.reporter(reportTime, "bus_factor", c.class, &c.busFactor)
	return nil
}

// TestChangeReporter modifies the reporter for unit tests.
func (c *Report) TestChangeReporter(r func(n time.Time, class string, name string, stat *HealthStat)) {
	c.reporter = r
}

// GetClass return with the class instance name (like last_net or country).
func (c *Report) GetClass() string {
	return c.class
}

// classID is a fork level short identifier for each class.
type classID int32

// ObserverFork is the durability calculator for each segment range.
type ObserverFork struct {
	controlledByClassCache []int32

	healthStat     []HealthStat
	busFactor      HealthStat
	busFactorCache []int32

	reportThreshold    int
	busFactorThreshold int

	classified []classID
}

// Process implements rangedloop.Partial.
func (c *ObserverFork) Process(ctx context.Context, segments []rangedloop.Segment) (err error) {
	controlledByClass := c.controlledByClassCache
	for i := range segments {
		s := &segments[i]

		if s.Inline() {
			continue
		}
		healthyPieceCount := 0
		for _, piece := range s.AliasPieces {
			if len(c.classified) <= int(piece.Alias) {
				// this is a new node, but we can ignore it.
				// will be included in the next execution cycle.
				continue
			}

			class := c.classified[piece.Alias]

			// unavailable/offline nodes were not classified
			if class == 0 {
				continue
			}

			healthyPieceCount++
			controlledByClass[class]++

		}

		busFactorGroups := c.busFactorCache

		streamLocation := s.StreamID.String() + "/" + strconv.FormatUint(s.Position.Encode(), 10)
		for classID, count := range controlledByClass {
			if count == 0 {
				continue
			}

			// reset the value for the next iteration
			controlledByClass[classID] = 0

			diff := healthyPieceCount - int(count)

			busFactorGroups = append(busFactorGroups, count)

			if c.reportThreshold > 0 && diff > c.reportThreshold {
				continue
			}

			c.healthStat[classID].Update(diff, streamLocation)
		}

		slices.SortFunc(busFactorGroups, func(a int32, b int32) int {
			return int(b - a)
		})
		rollingSum := 0
		busFactor := 0
		for _, count := range busFactorGroups {
			if rollingSum < c.busFactorThreshold {
				busFactor++
				rollingSum += int(count)
			} else {
				break
			}
		}

		c.busFactor.Update(busFactor, streamLocation)
	}
	return nil
}

func reportToEventkit(n time.Time, class string, name string, stat *HealthStat) {
	ek.Event("durability",
		eventkit.String("class", class),
		eventkit.String("value", name),
		eventkit.String("exemplar", stat.Exemplar),
		eventkit.Timestamp("report_time", n),
		eventkit.Int64("min", int64(stat.Min())),
	)
}

var _ rangedloop.Observer = &Report{}
var _ rangedloop.Partial = &ObserverFork{}
