// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/testrand"
	"storj.io/storj/storagenode/trust"
)

func TestSourcesConfig(t *testing.T) {
	var sources trust.Sources
	assert.Equal(t, "trust-sources", sources.Type())
	assert.Equal(t, "", sources.String())

	// Assert that comma separated sources can be set
	require.NoError(t, sources.Set("some-file,12ugxzSYW7UA1n7jfVyJHV2zL5TL1x5fSYZb48y7jZ1r3bLhbfo@127.0.0.1:12345"))
	assert.Equal(t, "some-file,12ugxzSYW7UA1n7jfVyJHV2zL5TL1x5fSYZb48y7jZ1r3bLhbfo@127.0.0.1:12345", sources.String())

	// Assert that a failure to set does not modify the current sources
	require.Error(t, sources.Set("@"))
	assert.Equal(t, "some-file,12ugxzSYW7UA1n7jfVyJHV2zL5TL1x5fSYZb48y7jZ1r3bLhbfo@127.0.0.1:12345", sources.String())

	// Assert the source was sources correctly
	if assert.Len(t, sources, 2) {
		assert.Equal(t, "some-file", sources[0].String())
		assert.Equal(t, "12ugxzSYW7UA1n7jfVyJHV2zL5TL1x5fSYZb48y7jZ1r3bLhbfo@127.0.0.1:12345", sources[1].String())
	}
}

func TestExclusionsConfig(t *testing.T) {
	exclusion0 := fmt.Sprintf("%s@", testrand.NodeID())
	exclusion1 := "foo.test"
	exclusion2 := fmt.Sprintf("%s@bar.test:7777", testrand.NodeID())

	exclusionConfig := strings.Join([]string{exclusion0, exclusion1, exclusion2}, ",")

	var exclusions trust.Exclusions
	assert.Equal(t, "trust-exclusions", exclusions.Type())
	assert.Equal(t, "", exclusions.String())

	// Assert that comma separated exclusions can be set
	require.NoError(t, exclusions.Set(exclusionConfig))
	assert.Equal(t, exclusionConfig, exclusions.String())

	// Assert that a failure to set does not modify the current exclusions
	require.Error(t, exclusions.Set("foo.test:7777"))
	assert.Equal(t, exclusionConfig, exclusions.String())

	// Assert the source was sources correctly
	if assert.Len(t, exclusions.Rules, 3) {
		assert.Equal(t, exclusion0, exclusions.Rules[0].String())
		assert.Equal(t, exclusion1, exclusions.Rules[1].String())
		assert.Equal(t, exclusion2, exclusions.Rules[2].String())
	}
}
