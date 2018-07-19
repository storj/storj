// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

// Beta function first attempt for the Reputation function
// An implementation based on http://folk.uio.no/josang/papers/JI2002-Bled.pdf
func Beta(recallBad float64, recallGood float64, weightDenomiator float64, featureGoodCount float64, featureCount float64, featureSum float64, featureCurrent float64) float64 {
	alpha := 1.0
	beta := 1.0

	// weight for update rule
	weight := featureCount / weightDenomiator

	r := weight * (1.0 + featureCurrent) / 2.0
	s := weight * (1.0 - featureCurrent) / 2.0

	alpha = alpha*recallGood + r
	beta = beta*recallBad + s

	newRep := alpha / (alpha + beta)

	return newRep
}
