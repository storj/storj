// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package repair

import (
	"math"
)

// SegmentHealth returns a value corresponding to the health of a segment
// in the repair queue. Lower health segments should be repaired first.
func SegmentHealth(numHealthy, minPieces int, failureRate float64) float64 {
	return 1.0 / SegmentDanger(numHealthy, minPieces, failureRate)
}

// SegmentDanger returns the chance of a segment with the given minPieces
// and the given number of healthy pieces of being lost in the next time
// period.
//
// It assumes:
//
// * Nodes fail at the given failureRate (i.e., each node has a failureRate
//   chance of going offline within the next time period).
// * Node failures are entirely independent. Obviously this is not the case,
//   because many nodes may be operated by a single entity or share network
//   infrastructure, in which case their failures would be correlated. But we
//   can't easily model that, so our best hope is to try to avoid putting
//   pieces for the same segment on related nodes to maximize failure
//   independence.
//
// (The "time period" we are talking about here could be anything. The returned
// danger value will be given in terms of whatever time period was used to
// determine failureRate. If it simplifies things, you can think of the time
// period as "one repair worker iteration".)
//
// If those things are true, then the number of nodes holding this segment
// that will go offline follows the Binomial distribution:
//
//     X ~ Binom(numHealthy, failureRate)
//
// A segment is lost if the number of nodes that go offline is higher than
// (numHealthy - minPieces). So we want to find
//
//     Pr[X > (numHealthy - minPieces)]
//
// If we invert the logic here, we can use the standard CDF for the binomial
// distribution.
//
//     Pr[X > (numHealthy - minPieces)] = 1 - Pr[X <= (numHealthy - minPieces)]
//
// And that gives us the danger value.
func SegmentDanger(numHealthy, minPieces int, failureRate float64) float64 {
	return 1.0 - binomialCDF(float64(numHealthy-minPieces), float64(numHealthy), failureRate)
}

// math.Lgamma without the returned sign parameter; it's unneeded here.
func lnGamma(x float64) float64 {
	lg, _ := math.Lgamma(x)
	return lg
}

// The following functions are based on code from
// Numerical Recipes in C, Second Edition, Section 6.4 (pp. 227-228).

// betaI calculates the incomplete beta function I_x(a, b).
func betaI(a, b, x float64) float64 {
	if x < 0.0 || x > 1.0 {
		return math.NaN()
	}
	bt := 0.0
	if x > 0.0 && x < 1.0 {
		// factors in front of the continued function
		bt = math.Exp(lnGamma(a+b) - lnGamma(a) - lnGamma(b) + a*math.Log(x) + b*math.Log(1.0-x))
	}
	if x < (a+1.0)/(a+b+2.0) {
		// use continued fraction directly
		return bt * betaCF(a, b, x) / a
	}
	// use continued fraction after making the symmetry transformation
	return 1.0 - bt*betaCF(b, a, 1.0-x)/b
}

const (
	// unlikely to go this far, as betaCF is expected to converge quickly for
	// typical values.
	maxIter = 100

	// betaI outputs will be accurate to within this amount.
	epsilon = 1.0e-14
)

// betaCF evaluates the continued fraction for the incomplete beta function
// by a modified Lentz's method.
func betaCF(a, b, x float64) float64 {
	avoidZero := func(f float64) float64 {
		if math.Abs(f) < math.SmallestNonzeroFloat64 {
			return math.SmallestNonzeroFloat64
		}
		return f
	}

	qab := a + b
	qap := a + 1.0
	qam := a - 1.0
	c := 1.0
	d := 1.0 / avoidZero(1.0-qab*x/qap)
	h := d

	for m := 1; m <= maxIter; m++ {
		m := float64(m)
		m2 := 2.0 * m
		aa := m * (b - m) * x / ((qam + m2) * (a + m2))
		// one step (the even one) of the recurrence
		d = 1.0 / avoidZero(1.0+aa*d)
		c = avoidZero(1.0 + aa/c)
		h *= d * c
		aa = -(a + m) * (qab + m) * x / ((a + m2) * (qap + m2))
		// next step of the recurrence (the odd one)
		d = 1.0 / avoidZero(1.0+aa*d)
		c = avoidZero(1.0 + aa/c)
		del := d * c
		h *= del
		if math.Abs(del-1.0) < epsilon {
			return h
		}
	}
	// a or b too big, or maxIter too small
	return math.NaN()
}

// binomialCDF evaluates the CDF of the binomial distribution Binom(n, p) at k.
// This is done using (1-p)**(n-k) when k is 0, or with the incomplete beta
// function otherwise.
func binomialCDF(k, n, p float64) float64 {
	k = math.Floor(k)
	if k < 0.0 || n < k {
		return math.NaN()
	}
	if k == n {
		return 1.0
	}
	if k == 0 {
		return math.Pow(1.0-p, n-k)
	}
	return betaI(n-k, k+1.0, 1.0-p)
}
