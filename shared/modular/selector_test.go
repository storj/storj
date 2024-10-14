// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package modular_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/private/mud"
	"storj.io/storj/shared/modular"
)

type ServiceA struct{}
type ServiceB struct{}
type ServiceC struct{}

func TestCreateSelector(t *testing.T) {
	ball := mud.NewBall()
	mud.Supply(ball, ServiceA{})
	mud.Supply(ball, ServiceB{})
	mud.Tag[ServiceB, modular.Service](ball, modular.Service{})
	mud.Supply(ball, ServiceC{})
	mud.Tag[ServiceC, modular.Service](ball, modular.Service{})

	var components []*mud.Component
	err := mud.ForEachDependency(ball, modular.CreateSelectorFromString(ball, "modular_test.ServiceA"), func(c *mud.Component) error {
		components = append(components, c)
		return nil
	}, mud.All)
	require.NoError(t, err)
	require.Len(t, components, 1)

	components = components[:0]
	err = mud.ForEachDependency(ball, modular.CreateSelectorFromString(ball, "service"), func(c *mud.Component) error {
		components = append(components, c)
		return nil
	}, mud.All)
	require.NoError(t, err)
	require.Len(t, components, 2)

	components = components[:0]
	err = mud.ForEachDependency(ball, modular.CreateSelectorFromString(ball, "service,-modular_test.ServiceB"), func(c *mud.Component) error {
		components = append(components, c)
		return nil
	}, mud.All)
	require.NoError(t, err)
	require.Len(t, components, 1)
	require.Equal(t, "modular_test.ServiceC", components[0].GetTarget().String())

}
