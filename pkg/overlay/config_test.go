// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
package overlay

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRun(t *testing.T) {
	ctx := context.Background()

	// run with nil, pass pointer to Kademlia in context
	err := Config{}.Run(ctx, nil)
	assert.Error(t, err)
	assert.Equal(t, "overlay error: unable to get master db instance", err.Error())
}
