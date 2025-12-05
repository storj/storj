// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

// Rule indicates whether or not a Satellite URL is trusted.
type Rule interface {
	// IsTrusted returns true if the given Satellite is trusted and false otherwise
	IsTrusted(url SatelliteURL) bool

	// String returns a string representation of the rule
	String() string
}

// Rules is a collection of rules.
type Rules []Rule

// IsTrusted returns true if the given Satellite is trusted and false otherwise.
func (rules Rules) IsTrusted(url SatelliteURL) bool {
	for _, rule := range rules {
		if !rule.IsTrusted(url) {
			return false
		}
	}
	return true
}
