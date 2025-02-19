// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package root

import (
	"testing"

	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/mud"
)

func TestModule(t *testing.T) {
	ball := mud.NewBall()

	// this will panic, in case of any very bad module definition
	Module(ball)

	// TODO: would be better to keep the definition here, but it's not yet possible due to circular dependencies...
	modular.CreateSelectorFromString(ball, "@hashstore")
}
