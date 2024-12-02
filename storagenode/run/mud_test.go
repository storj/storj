package root

import (
	"storj.io/storj/private/mud"
	"storj.io/storj/shared/modular"
	"testing"
)

func TestModule(t *testing.T) {
	ball := mud.NewBall()

	// this will panic, in case of any very bad module definition
	Module(ball)

	// TODO: would be better to keep the definiton here, but it's not yet possible due to circular dependencies...
	modular.CreateSelectorFromString(ball, "@hashstore")
}
