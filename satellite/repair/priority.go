// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package repair

import "math"

// SegmentHealth returns a value corresponding to the health of a segment in the
// repair queue. Lower health segments should be repaired first.
//
// This calculation purports to find the number of iterations for which a
// segment can be expected to survive, with the given failureRate. The number of
// iterations for the segment to survive (X) can be modeled with the negative
// binomial distribution, with the number of pieces that must be lost as the
// success threshold r, and the chance of losing a single piece in a round as
// the trial success probability p.
//
// First, we calculate the expected number of iterations for a segment to
// survive if we were to lose exactly one node every iteration:
//
//	r = numHealthy - minPieces + 1
//	p = (totalNodes - numHealthy) / totalNodes
//	X ~ NB(r, p)
//
// Then we take the mean of that distribution to use as our expected value,
// which is pr/(1-p).
//
// Finally, to get away from the "one node per iteration" simplification, we
// just scale the magnitude of the iterations in the model so that there really
// is one node being lost. For example, if our failureRate and totalNodes imply
// a churn rate of 3 nodes per day, we just take 1/3 of a day and call that an
// "iteration" for purposes of the model. To convert iterations in the model to
// days, we divide the mean of the negative binomial distribution (X, above) by
// the number of nodes that we estimate will churn in one day.
func SegmentHealth(numHealthy, minPieces, totalNodes int, failureRate float64) float64 {
	if totalNodes < minTotalNodes {
		// this model gives wonky results when there are too few nodes; pretend
		// there are more nodes than there really are so that the model gives
		// sane repair priorities
		totalNodes = minTotalNodes
	}
	churnPerRound := float64(totalNodes) * failureRate
	if churnPerRound < minChurnPerRound {
		// we artificially limit churnPerRound from going too low in cases
		// where there are not many nodes, so that health values do not
		// start to approach the floating point maximum
		churnPerRound = minChurnPerRound
	}
	p := float64(totalNodes-numHealthy) / float64(totalNodes)
	if p == 1.0 {
		// floating point precision is insufficient to represent the difference
		// from p to 1. there are too many nodes for this model, or else
		// numHealthy is 0 somehow. we can't proceed with the normal calculation
		// or we will divide by zero.
		return math.Inf(1)
	}
	mean1 := float64(numHealthy-minPieces+1) * p / (1 - p)
	return mean1 / churnPerRound
}

const (
	minChurnPerRound = 1e-10
	minTotalNodes    = 100
)
