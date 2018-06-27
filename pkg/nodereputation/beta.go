package nodereputation

// BetaReturn Result type from Beta function
type BetaReturn struct {
	Reputation float64
	Mean       float64
}

// Beta function
func Beta(recallBad float64, recallGood float64, weightDenomiator float64, featureCount float64, featureSum float64, featureCurrent float64) BetaReturn {
	alpha := 1.0
	beta := 1.0

	// weight for update rule
	weight := featureCount / weightDenomiator

	r := weight * (1.0 + featureCurrent) / 2.0
	s := weight * (1.0 - featureCurrent) / 2.0

	alpha = alpha*recallGood + r
	beta = beta*recallBad + s

	newRep := alpha / (alpha + beta)
	meanRep := featureSum / featureCount

	return BetaReturn{
		reputation: newRep,
		mean:       meanRep,
	}
}
