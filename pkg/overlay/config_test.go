// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
package overlay

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/kademlia"
)

func TestRun(t *testing.T) {
	bctx := context.Background()

	kad := &kademlia.Kademlia{}
	var kadKey kademlia.CtxKey
	ctx := context.WithValue(bctx, kadKey, kad)

	// run with nil
	err := Config{}.Run(context.Background(), nil)
	assert.Error(t, err)
	assert.Equal(t, "overlay error: programmer error: kademlia responsibility unstarted", err.Error())

	// run with nil, pass pointer to Kademlia in context
	err = Config{}.Run(ctx, nil)
	assert.Error(t, err)
	assert.Equal(t, "overlay error: unable to get master db instance", err.Error())
}
