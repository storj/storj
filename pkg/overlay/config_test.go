// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
package overlay

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/statdb"
)

func TestRun(t *testing.T) {
	bctx := context.Background()

	kad := &kademlia.Kademlia{}
	var kadKey kademlia.CtxKey
	ctx := context.WithValue(bctx, kadKey, kad)

	sdb := &statdb.StatDB{}
	var statKey statdb.CtxKey
	ctx = context.WithValue(ctx, statKey, sdb)

	// run with nil
	err := Config{}.Run(context.Background(), nil)
	assert.Error(t, err)
	assert.Equal(t, "overlay error: programmer error: kademlia responsibility unstarted", err.Error())

	// run with nil, pass pointer to Kademlia in context
	err = Config{}.Run(ctx, nil)
	assert.Error(t, err)
	assert.Equal(t, "overlay error: database scheme not supported: ", err.Error())

	// db scheme redis conn fail
	err = Config{DatabaseURL: "redis://somedir/overlay.db/?db=1"}.Run(ctx, nil)

	assert.Error(t, err)
	assert.Equal(t, "redis error: ping failed: dial tcp: address somedir: missing port in address", err.Error())

	// db scheme bolt conn fail
	err = Config{DatabaseURL: "bolt://somedir/overlay.db"}.Run(ctx, nil)
	assert.Error(t, err)
}
