// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package reputation

import "math"

// UpdateReputationMultiple works like UpdateReputation, but applies multiple
// successive counts of an event type to the alpha and beta measures.
//
// With the arguments as named, applies 'count' successful audits. To apply negative
// audits, swap the alpha and beta parameters and return values.
//
// WARNING: GREEK LETTER MATH AHEAD
//
// Applying n successful audit results to an initial alpha value of α₀ gives a
// new α₁ value of:
//
//	α₁ = λⁿα₀ + λⁿ⁻¹w + λⁿ⁻²w + ... + λ²w + λw + w
//
// The terms with w are the first n terms of a geometric series with coefficient
// w and common ratio λ. The closed form formula for the sum of those first n
// terms is (w(1-λⁿ) / (1-λ))
// (https://en.wikipedia.org/wiki/Geometric_series#Closed-form_formula).
// Adding the initial λⁿα₀ term, we get
//
//	α₁ = λⁿα₀ + w(1-λⁿ) / (1-λ)
//
// The formula has the same structure for beta for n _failures_.
//
//	β₁ = λⁿβ₀ + w(1-λⁿ) / (1-λ)
//
// For n _failures_,
//
//	α₁ = λⁿα₀
//
// For n _successes_,
//
//	β₁ = λⁿβ₀
func UpdateReputationMultiple(count int, alpha, beta, lambda, w float64) (newAlpha, newBeta float64) {
	if lambda == 1 {
		// special case: when the coefficient is 1, the closed-form formula is invalid
		// (gives NaN because of a division by zero). Fortunately, the replacement
		// formula in this case is even simpler.
		newAlpha = alpha + w*float64(count)
		newBeta = beta
	} else {
		lambdaPowN := math.Pow(lambda, float64(count))
		newAlpha = lambdaPowN*alpha + w*(1-lambdaPowN)/(1-lambda)
		newBeta = lambdaPowN * beta
	}
	return newAlpha, newBeta
}
