// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package modular_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/mud"
)

func TestCreateSelector(t *testing.T) {
	createModule := func() *mud.Ball {
		ball := mud.NewBall()
		mud.Provide[DiggingService](ball, NewDiggingService)
		mud.Tag[DiggingService, modular.Service](ball, modular.Service{})
		mud.Provide[Whetstone](ball, NewWhetstone)
		mud.Provide[ScythingService](ball, NewScythingService)
		mud.Tag[ScythingService, modular.Service](ball, modular.Service{})
		mud.Provide[RebellionService](ball, NewRevolutionService)
		mud.Provide[Scythe](ball, NewScythe)
		mud.Provide[Hoe](ball, NewHoe)
		mud.RegisterInterfaceImplementation[Tool, Hoe](ball)
		return ball
	}

	t.Run("select component", func(t *testing.T) {
		var components []*mud.Component
		ball := createModule()
		err := mud.ForEachDependency(ball, modular.CreateSelectorFromString(ball, "modular_test.DiggingService"), func(c *mud.Component) error {
			components = append(components, c)
			return nil
		}, mud.All)
		require.NoError(t, err)
		// only digging service
		require.Len(t, components, 1)
	})

	t.Run("tagged selection", func(t *testing.T) {
		var components []*mud.Component
		ball := createModule()

		err := mud.ForEachDependency(ball, modular.CreateSelectorFromString(ball, "service"), func(c *mud.Component) error {
			components = append(components, c)
			return nil
		}, mud.All)
		require.NoError(t, err)
		// digging + scything + the whetstone dependency
		require.Len(t, components, 3)
	})

	t.Run("exclusion", func(t *testing.T) {
		var components []*mud.Component
		ball := createModule()

		err := mud.ForEachDependency(ball, modular.CreateSelectorFromString(ball, "service,-modular_test.ScythingService"), func(c *mud.Component) error {
			components = append(components, c)
			return nil
		}, mud.All)
		require.NoError(t, err)
		// only digging
		require.Len(t, components, 1)
		require.Equal(t, "modular_test.DiggingService", components[0].GetTarget().String())
	})

	t.Run("interface selection", func(t *testing.T) {
		var components []*mud.Component
		ctx := testcontext.New(t)
		ball := createModule()

		components = components[:0]
		// scythe is proven to be more useful during a rebellion: https://en.wikipedia.org/wiki/Gy%C3%B6rgy_D%C3%B3zsa
		// use that one
		err := mud.ForEachDependency(ball, modular.CreateSelectorFromString(ball, "modular_test.RebellionService,modular_test.Tool=modular_test.Scythe"), func(c *mud.Component) error {
			components = append(components, c)
			_ = c.Init(ctx)
			return nil
		}, mud.All)
		require.NoError(t, err)
		// only rebellion + scythe + tool interface
		require.Len(t, components, 3)
		rebellion := mud.MustLookup[RebellionService](ball)
		require.Contains(t, rebellion.Tool.Use(), "scythe")
	})

	t.Run("default implementation", func(t *testing.T) {
		var components []*mud.Component
		ball := createModule()
		ctx := testcontext.New(t)

		// default tool is hoe (due to mud.RegisterInterfaceImplementation), if you don't specify any (good luck)
		err := mud.ForEachDependency(ball, modular.CreateSelectorFromString(ball, "modular_test.RebellionService"), func(c *mud.Component) error {
			components = append(components, c)
			_ = c.Init(ctx)
			return nil
		}, mud.All)
		require.NoError(t, err)
		// only rebellion + hoe + tool interface
		require.Len(t, components, 3)
		rebellion := mud.MustLookup[RebellionService](ball)
		require.Contains(t, rebellion.Tool.Use(), "hoe")
	})

	t.Run("disable implementation", func(t *testing.T) {
		var components []*mud.Component
		ball := createModule()
		ctx := testcontext.New(t)

		// violence free rebellion without tool
		err := mud.ForEachDependency(ball, modular.CreateSelectorFromString(ball, "modular_test.RebellionService,!modular_test.Tool"), func(c *mud.Component) error {
			components = append(components, c)
			_ = c.Init(ctx)
			return nil
		}, mud.All)
		require.NoError(t, err)
		// only rebellion + tool interface
		require.Len(t, components, 1)
		rebellion := mud.MustLookup[RebellionService](ball)
		require.Nil(t, rebellion.Tool)
	})

}

type DiggingService struct{}

func NewDiggingService() DiggingService {
	return DiggingService{}
}

type ScythingService struct {
	Whetstone Whetstone
}

func NewScythingService(w Whetstone) ScythingService {
	return ScythingService{
		Whetstone: Whetstone{},
	}
}

type RebellionService struct {
	Tool Tool
}

func NewRevolutionService(t Tool) RebellionService {
	return RebellionService{
		Tool: t,
	}
}

type Whetstone struct {
}

func NewWhetstone() Whetstone {
	return Whetstone{}
}

type Tool interface {
	Use() string
}

type Scythe struct{}

func NewScythe() Scythe {
	return Scythe{}
}

func (s Scythe) Use() string {
	return "scythe is doing its job"
}

type Hoe struct{}

func NewHoe() Hoe {
	return Hoe{}
}

func (h Hoe) Use() string {
	return "hoe is doing its job"
}
