// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/storagenode/trust"
)

func TestRulesIsTrusted(t *testing.T) {

	url := makeSatelliteURL("domain.test")

	// default is trusted when there are no rules
	var rules trust.Rules
	assert.True(t, rules.IsTrusted(url))

	rules = trust.Rules{fakeRule(true)}
	assert.True(t, rules.IsTrusted(url))

	rules = trust.Rules{fakeRule(false)}
	assert.False(t, rules.IsTrusted(url))

	rules = trust.Rules{fakeRule(true), fakeRule(false), fakeRule(true)}
	assert.False(t, rules.IsTrusted(url))
}

type fakeRule bool

func (rule fakeRule) IsTrusted(url trust.SatelliteURL) bool {
	return bool(rule)
}

func (rule fakeRule) String() string {
	return "fake"
}
