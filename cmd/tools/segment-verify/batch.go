// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"sort"

	"go.uber.org/zap"

	"storj.io/storj/satellite/metabase"
)

// CreateBatches creates load-balanced queues of segments to verify.
func (service *Service) CreateBatches(ctx context.Context, segments []*Segment) (_ []*Batch, err error) {
	defer mon.Task()(&ctx)(&err)

	if len(segments) == 0 {
		return nil, Error.New("no segments")
	}

	// Remove offline nodes and prioritize nodes.
	for _, segment := range segments {
		service.selectOnlinePieces(segment)
		service.sortPriorityToFirst(segment)
	}

	batches := map[metabase.NodeAlias]*Batch{}
	enqueue := func(alias metabase.NodeAlias, segment *Segment) {
		q, ok := batches[alias]
		if !ok {
			q = &Batch{Alias: alias}
			batches[alias] = q
		}
		q.Items = append(q.Items, segment)
	}

	// Distribute things randomly into batches.
	// We assume that segment.AliasPieces is randomly ordered in terms of nodes.
	for _, segment := range segments {
		if len(segment.AliasPieces) < int(segment.Status.Retry) {
			if service.config.Check == 0 {
				// some pieces were removed in selectOnlinePieces. adjust the expected count.
				segment.Status.Retry = int32(len(segment.AliasPieces))
			} else {
				service.log.Error("segment contains too few pieces. skipping segment",
					zap.Int("num-pieces", len(segment.AliasPieces)),
					zap.Int32("expected", segment.Status.Retry),
					zap.Stringer("stream-id", segment.StreamID),
					zap.Uint64("position", segment.Position.Encode()))
				continue
			}
		}
		for _, piece := range segment.AliasPieces[:segment.Status.Retry] {
			enqueue(piece.Alias, segment)
		}
	}

	allQueues := []*Batch{}
	for _, q := range batches {
		allQueues = append(allQueues, q)
	}
	// sort queues by queue length descending.
	sort.Slice(allQueues, func(i, k int) bool {
		return allQueues[i].Len() >= allQueues[k].Len()
	})

	// try to redistribute segments in different slices
	//   queue with length above 65% will be redistributed
	//   to queues with length below 40%, but no more than 50%
	highIndex := len(allQueues) * (100 - 65) / 100
	midIndex := len(allQueues) * (100 - 50) / 100
	lowIndex := len(allQueues) * (100 - 40) / 100

	midLen := allQueues[midIndex].Len()
	highLen := allQueues[highIndex].Len()

	smallBatches := mapFromBatches(allQueues[lowIndex:])

	// iterate over all queues
	for _, large := range allQueues[:highIndex] {
		// don't redistribute priority nodes
		if service.priorityNodes.Contains(large.Alias) {
			continue
		}

		newItems := large.Items[:highLen]

	nextSegment:
		for _, segment := range large.Items[highLen:] {
			// try to find a piece that can be moved into a small batch.
			for _, piece := range segment.AliasPieces[segment.Status.Retry:] {
				if q, ok := smallBatches[piece.Alias]; ok {
					// move to the other queue
					q.Items = append(q.Items, segment)
					if q.Len() >= midLen {
						delete(smallBatches, piece.Alias)
					}
					continue nextSegment
				}
			}

			// keep the segment in the slice,
			// because we didn't find a good alternative
			newItems = append(newItems, segment)
		}
		large.Items = newItems

		if len(smallBatches) == 0 {
			break
		}
	}

	return allQueues, nil
}

// selectOnlinePieces modifies slice such that it only contains online pieces.
func (service *Service) selectOnlinePieces(segment *Segment) {
	for i, x := range segment.AliasPieces {
		if !service.offlineNodes.Contains(x.Alias) && !service.ignoreNodes.Contains(x.Alias) {
			continue
		}

		// found an offline node, start removing
		rs := segment.AliasPieces[:i]
		for _, x := range segment.AliasPieces[i+1:] {
			if !service.offlineNodes.Contains(x.Alias) && !service.ignoreNodes.Contains(x.Alias) {
				rs = append(rs, x)
			}
		}
		segment.AliasPieces = rs
		return
	}
}

// removePriorityPieces modifies slice such that it only contains non-priority pieces.
func (service *Service) removePriorityPieces(segment *Segment) {
	target := 0
	for _, x := range segment.AliasPieces {
		if service.priorityNodes.Contains(x.Alias) {
			continue
		}
		segment.AliasPieces[target] = x
		target++
	}
	segment.AliasPieces = segment.AliasPieces[:target]
}

// sortPriorityToFirst moves priority node pieces at the front of the list.
func (service *Service) sortPriorityToFirst(segment *Segment) {
	xs := segment.AliasPieces
	target := 0
	for i, x := range xs {
		if service.priorityNodes.Contains(x.Alias) {
			xs[target], xs[i] = xs[i], xs[target]
			target++
		}
	}
}

// mapFromBatches creates a map from the specified batches.
func mapFromBatches(batches []*Batch) map[metabase.NodeAlias]*Batch {
	xs := make(map[metabase.NodeAlias]*Batch, len(batches))
	for _, b := range batches {
		xs[b.Alias] = b
	}
	return xs
}
