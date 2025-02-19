// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mud_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/mud"
)

type Store interface {
	Hello() string
}

type Fancy struct {
}

func NewFancy() *Fancy {
	return &Fancy{}
}

func (f *Fancy) Hello() string {
	return "fancy"
}

var _ Store = &Fancy{}

type Boring struct {
}

func NewBoring() *Boring {
	return &Boring{}
}

func (b *Boring) Hello() string {
	return "boring"
}

var _ Store = &Boring{}

func TestInterfacing(t *testing.T) {
	initball := func() *mud.Ball {
		ball := mud.NewBall()
		mud.Provide[*Fancy](ball, NewFancy)
		mud.Provide[*Boring](ball, NewBoring)
		mud.RegisterInterfaceImplementation[Store, *Fancy](ball)
		return ball
	}

	// by default, we use fancy implementation
	t.Run("default", func(t *testing.T) {

		ball := initball()

		// create all instances
		err := mud.ForEachDependency(ball, mud.All, mud.Initialize(testcontext.New(t)))
		require.NoError(t, err)

		err = mud.Execute0(testcontext.New(t), ball, func(s Store) {
			require.Equal(t, "fancy", s.Hello())
		})
		require.NoError(t, err)
	})

	// we can change to an alternative implementation
	t.Run("changed", func(t *testing.T) {

		ball := initball()
		mud.ReplaceDependency[Store, *Boring](ball)

		// create all instances
		err := mud.ForEachDependency(ball, mud.All, mud.Initialize(testcontext.New(t)))
		require.NoError(t, err)

		err = mud.Execute0(testcontext.New(t), ball, func(s Store) {
			require.Equal(t, "boring", s.Hello())
		})
		require.NoError(t, err)
	})

	// or, we can disable implementation all together
	t.Run("disabled", func(t *testing.T) {

		ball := initball()
		mud.DisableImplementation[Store](ball)

		// create all instances
		err := mud.ForEachDependency(ball, mud.All, mud.Initialize(testcontext.New(t)))
		require.NoError(t, err)

		err = mud.Execute0(testcontext.New(t), ball, func(s Store) {
			require.Nil(t, s)
		})
		require.NoError(t, err)
	})

}
