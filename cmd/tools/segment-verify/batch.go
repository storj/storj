// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"sort"

	"storj.io/storj/satellite/metabase"
)

// CreateBatches creates load-balanced queues of segments to verify.
func (service *Service) CreateBatches(segments []*Segment) ([]*Batch, error) {
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
	// We assume that segment.Pieces is randomly ordered in terms of nodes.
	for _, segment := range segments {
		if len(segment.Pieces) < VerifyPieces {
			panic("segment contains too few pieces")
		}
		for _, piece := range segment.Pieces[:VerifyPieces] {
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
	highIndex := len(allQueues) - len(allQueues)*65/100
	midIndex := len(allQueues) - len(allQueues)*50/100
	lowIndex := len(allQueues) - len(allQueues)*40/100

	midLen := allQueues[midIndex].Len()
	highLen := allQueues[highIndex].Len()

	smallBatches := mapFromBatches(allQueues[lowIndex:])

	// iterate over all queues
	for _, large := range allQueues[:highIndex] {
		// don't redistribute priority nodes
		if service.PriorityNodes.Contains(large.Alias) {
			continue
		}

		newItems := large.Items[:highLen]

	nextSegment:
		for _, segment := range large.Items[highLen:] {
			// try to find a piece that can be moved into a small batch.
			for _, piece := range segment.Pieces[VerifyPieces:] {
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
	for i, x := range segment.Pieces {
		if !service.OfflineNodes.Contains(x.Alias) {
			continue
		}

		// found an offline node, start removing
		rs := segment.Pieces[:i]
		for _, x := range segment.Pieces[i+1:] {
			if !service.OfflineNodes.Contains(x.Alias) {
				rs = append(rs, x)
			}
		}
		segment.Pieces = rs
		return
	}
}

// sortPriorityToFirst moves priority node pieces at the front of the list.
func (service *Service) sortPriorityToFirst(segment *Segment) {
	xs := segment.Pieces
	target := 0
	for i, x := range xs {
		if service.PriorityNodes.Contains(x.Alias) {
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
