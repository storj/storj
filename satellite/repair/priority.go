// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package repair

import "math"

// segmentHealthNHD purports to find the number of days that a segment can be
// expected to survive, with the given failureRate.
//
// The loss of nodes and pieces on the Storj network relative to a single
// segment can be modeled as a bin holding differently colored balls, one for
// each node on the network. Nodes holding pieces of our segment become red
// balls, and all other nodes are blue balls. One by one, we reach in and remove
// a ball from the bin at random, symbolizing that node going offline. If the
// ball is blue, we count that as a success (our segment is unaffected). If the
// ball is red, it is a failure (our segment has lost a piece). We want to know
// how many draws it will take before some number of failures is reached (that
// is, how long will it be before a segment loses too many pieces and is no
// longer reconstructible, if we don't repair it along the way).
//
// With this formulation, the problem is nearly identical to the situation
// described by the negative hypergeometric distribution
// (https://en.wikipedia.org/wiki/Negative_hypergeometric_distribution). It is
// related to the negative binomial distribution
// (https://en.wikipedia.org/wiki/Negative_binomial_distribution), but the NBD
// deals with drawing balls from a bin with replacement, while the NHD deals
// with drawing balls from a bin without replacement. Because we can't expect
// lost nodes to come back, we don't put balls back into the bin once they are
// drawn, so ours is a case of drawing without replacement. The negative
// hypergeometric distribution more closely matches our problem, especially
// around certain edge cases.
//
// Do nodes tend to go offline one after another like a sequence of balls being
// chosen from a bin? No, in reality node failures tend to happen in conjunction
// with other failures. They are not independent occurrences. However, if we
// have done a good enough job in declumping segments so that pieces tend to be
// well distributed across unrelated nodes, then node failure patterns from the
// point of view of a particular segment should be pretty indistinguishable from
// random and independent. We have measured in the past what appeared to be a
// fairly steady rate of node failure: each node has something like a 0.00005435
// chance of going permanently offline on any given day. With a population size
// of about 24k nodes, this gives us a mean time between failures (MTBF) for
// nodes of about 18.5 hours. We can make the simplifying assumption that nodes
// do in fact go offline at this rate.
//
// In our formulation of the model, the NHD parameters are:
//
//	N (the total number of balls) = totalNodes
//	K (the number of balls considered successes) = totalNodes-numHealthy
//	r (the number of failures until we're done) = numHealthy-minPieces+1
//
// Knowing this tells us how to calculate how many draws to expect segment decay
// to take. The expected value of the negative hypergeometric distribution (the
// mean value you would get if you tried the experiment enough times) is
// r*K/(N-K+1).
//
// Now, knowing the number of draws doesn't immediately tell us how many _days_
// of survival to expect. We use the failureRate parameter to get from _draws_
// to _days_.
//
// We want to scale things so that one draw corresponds to one node failure.
// All we need for that is the MTBF, the mean time between failures. One draw
// can correspond to one MTBF interval. Since we know that failureRate is a
// chance of failure per node per day, we can multiply it by totalNodes to get
// the total number of node failures per day, and invert that value to get the
// mean number of days per failure, and that is the MTBF.
//
// For more analysis of this model, see the Jupyter Notebook
// repair_and_durability/repairPriority/hypergeo.ipynb in the storj/datascience
// repository.
func segmentHealthNHD(numHealthy, minPieces, totalNodes int, failureRate float64) float64 {
	if numHealthy < minPieces {
		// take a shortcut.
		return 0
	}
	N := float64(totalNodes)                 // the total population
	K := float64(totalNodes - numHealthy)    // the number of successes/blue balls in the bin
	r := float64(numHealthy - minPieces + 1) // how many failures before the segment is irrecoverable

	// the mean of the distribution, corresponding to the expected number of
	// successes before we reach r failures
	expectedNumberOfSuccesses := r * K / (N - K + 1)
	// the total number of expected draws, including both successes and failures
	expectedNumberOfDraws := expectedNumberOfSuccesses + r

	drawsPerDay := N * failureRate
	mtbf := 1 / drawsPerDay
	days := expectedNumberOfDraws * mtbf

	return days
}

const (
	// These somewhat magic-looking values correspond to pop-significance values
	// suggested by @elek in the context of a different health model. They have
	// been adapted to work in this model.
	popSignificanceLow  = 3154 // from segmentHealthNHD(34, 29, 24000, 0.00005435)
	popSignificanceHigh = 5385 // from segmentHealthNHD(40, 29, 24000, 0.00005435)
)

// SegmentHealth returns a value corresponding to the health of a segment in the
// repair queue. Lower health segments should be repaired first.
//
// This implementation uses segmentHealthNHD to calculate the base health value.
//
// An additional wrinkle added here is that we need to assign high priority to
// pieces which need to be repaired as soon as possible, e.g., pieces out of
// placement ("POPs"). We want to tune it so that segments with POPs are
// generally higher priority than other segments, and segments with more POPs
// are generally higher priority than segments with fewer POPs. It is possible,
// however, for a segment with no POPs to be prioritized above a segment that
// does have POPs, if the first segment is in sufficient danger and the second
// segment is not.
func SegmentHealth(numHealthy, minPieces, totalNodes int, failureRate float64, numForcingRepair int) float64 {
	base := segmentHealthNHD(numHealthy, minPieces, totalNodes, failureRate)

	if numForcingRepair > 0 {
		// POP segments are put between segments with lifetimes between popSignificanceLow and popSignificanceHigh days.
		popSignificance := math.Min(float64(numForcingRepair)/float64(minPieces), 1)
		return math.Min(base, popSignificanceHigh-(popSignificanceHigh-popSignificanceLow)*popSignificance)
	}
	return base
}
