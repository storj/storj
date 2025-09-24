// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeselection

import (
	"context"
	"math"
	"sort"

	"github.com/jtolio/mito"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
)

// DownloadSelector will take a map of possible nodes to choose for a download.
// It returns a new map of nodes to consider for selecting for the download.
// It is always true that 0 <= len(result) <= len(possibleNodes), and every
// element in result will have come from possibleNodes. 'needed' is a hint to
// the selector of how many nodes are needed for return ideally, so many
// selectors will try to return at least 'needed' nodes.
type DownloadSelector func(ctx context.Context, requester storj.NodeID, possibleNodes map[storj.NodeID]*SelectedNode, needed int) (map[storj.NodeID]*SelectedNode, error)

// DownloadSelectorFromString parses complex node download selection expressions
// from config lines.
func DownloadSelectorFromString(expr string, environment PlacementConfigEnvironment) (DownloadSelector, error) {
	if expr == "" {
		expr = "random"
	}
	env := map[any]any{
		"random":    DefaultDownloadSelector,
		"choiceofn": DownloadChoiceOfN,
		"best":      DownloadBest,
	}
	environment.apply(env)
	selector, err := mito.Eval(expr, env)
	if err != nil {
		return nil, errs.New("Invalid download selector definition '%s', %v", expr, err)
	}
	return selector.(DownloadSelector), nil
}

// ExcludeAllDownloadSelector is a DownloadSelector that always returns an
// empty map.
var ExcludeAllDownloadSelector DownloadSelector = excludeAllDownloadSelector

var excludeAllDownloadSelectorTask = mon.Task()

func excludeAllDownloadSelector(ctx context.Context, _ storj.NodeID, _ map[storj.NodeID]*SelectedNode, _ int) (_ map[storj.NodeID]*SelectedNode, err error) {
	defer excludeAllDownloadSelectorTask(&ctx)(&err)

	return map[storj.NodeID]*SelectedNode{}, nil
}

// DefaultDownloadSelector is a DownloadSelector that returns the set of
// possibleNodes unchanged.
var DefaultDownloadSelector DownloadSelector = defaultDownloadSelector

var defaultDownloadSelectorTask = mon.Task()

func defaultDownloadSelector(ctx context.Context, _ storj.NodeID, possibleNodes map[storj.NodeID]*SelectedNode, _ int) (_ map[storj.NodeID]*SelectedNode, err error) {
	defer defaultDownloadSelectorTask(&ctx)(&err)
	return possibleNodes, nil
}

var downloadChoiceOfNTask = mon.Task()

// DownloadChoiceOfN will take a set of nodes and winnow it down using choice
// of n. n is an int64 type due to a mito scripting shortcoming but really an
// int16 should be fine.
func DownloadChoiceOfN(comparison CompareNodes, n int64) DownloadSelector {
	return func(ctx context.Context, requester storj.NodeID, possibleNodes map[storj.NodeID]*SelectedNode, needed int) (_ map[storj.NodeID]*SelectedNode, err error) {
		defer downloadChoiceOfNTask(&ctx)(&err)
		nodeSlice := make([]*SelectedNode, 0, len(possibleNodes)+needed)
		for _, node := range possibleNodes {
			nodeSlice = append(nodeSlice, node)
		}

		nodeSlice = choiceOfNReduction(ctx, comparison(requester), int(n), nodeSlice, needed)

		result := make(map[storj.NodeID]*SelectedNode, needed)
		for _, node := range nodeSlice {
			result[node.ID] = node
		}
		return result, nil
	}
}

var downloadBestTask = mon.Task()

// DownloadBest will take a set of nodes and will return just the best nodes.
func DownloadBest(tracker UploadSuccessTracker) DownloadSelector {
	return func(ctx context.Context, requester storj.NodeID, possibleNodes map[storj.NodeID]*SelectedNode, needed int) (_ map[storj.NodeID]*SelectedNode, err error) {
		defer downloadBestTask(&ctx)(&err)
		nodeSlice := make([]*SelectedNode, 0, len(possibleNodes)+needed)
		for _, node := range possibleNodes {
			nodeSlice = append(nodeSlice, node)
		}

		getSuccessRate := tracker.Get(requester)

		sort.Slice(nodeSlice, func(i, j int) bool {
			success0 := getSuccessRate(nodeSlice[i])
			success1 := getSuccessRate(nodeSlice[j])

			// we do the same thing as choiceofn where we assume NaN is better
			// than not NaN. this has the additional benefit of falling back
			// to random selection behavior for full nodes, where they still
			// get a shot.
			return success0 > success1 || math.IsNaN(success0) && !math.IsNaN(success1)
		})
		if len(nodeSlice) > needed {
			nodeSlice = nodeSlice[:needed]
		}

		result := make(map[storj.NodeID]*SelectedNode, needed)
		for _, node := range nodeSlice {
			result[node.ID] = node
		}
		return result, nil
	}
}
