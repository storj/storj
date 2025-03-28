// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package mudplanet

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/mud"
)

// Config is a configuration for the test environment.
type Config struct {
	Components []Component
}

// Modules is a list of modules that can be applied to a component.
type Modules []func(ball *mud.Ball)

// Component is a configuration for a single component in the test environment.
type Component struct {
	Name     string
	Type     reflect.Type
	Modules  Modules
	Selector mud.ComponentSelector

	// PreInit is a list of hooks that are executed before the component is initialized. Parameters will be injected by type.
	PreInit []any
}

// RuntimeEnvironment is the runtime environment of the test.
type RuntimeEnvironment struct {
	Services []Microservice
}

// Microservice is a single instance of a component in the test environment.
type Microservice struct {
	Name     string
	Index    int
	Ball     *mud.Ball
	Selector mud.ComponentSelector
	WorkDir  string
}

// Run sets up and executes a test environment with the specified components and configuration.
// It initializes all components, executes pre-init hooks, starts all services,
// calls the provided callback function, and ensures proper cleanup afterward.
func Run(t *testing.T, c Config, callback func(t *testing.T, ctx context.Context, run RuntimeEnvironment)) {
	tctx := testcontext.New(t)

	ctx, cancel := context.WithCancel(tctx)
	logger := zaptest.NewLogger(t)
	re := RuntimeEnvironment{}

	for ix, component := range c.Components {
		ball := mud.NewBall()

		{
			// default components, usually provided by the CLI Runner
			mud.Supply[*zap.Logger](ball, logger)
			mud.Supply[*identity.FullIdentity](ball, testidentity.MustPregeneratedIdentity(ix, storj.LatestIDVersion()))
			mud.View[*identity.FullIdentity, storj.NodeID](ball, func(fullIdentity *identity.FullIdentity) storj.NodeID {
				return fullIdentity.ID
			})
			mud.Supply[*modular.StopTrigger](ball, &modular.StopTrigger{Cancel: cancel})
			mud.Supply[*testing.T](ball, t)
		}

		// apply module customization
		for _, module := range component.Modules {
			module(ball)
		}

		// create
		microService := Microservice{
			Name:     component.Name,
			Index:    len(re.Services),
			Ball:     ball,
			Selector: component.Selector,
			WorkDir:  tctx.Dir(component.Name, strconv.Itoa(len(re.Services))),
		}

		// initialize and fill all the required configs (dependencies of the selector)
		err := InitConfigDefaults(ctx, t, ball, component.Selector, microService.WorkDir)
		require.NoError(t, err)

		// additional customization point before we init all the remaining components
		re.Services = append(re.Services, microService)
		for _, hook := range component.PreInit {
			err = initAndExec(ctx, ball, hook)
			require.NoError(t, err)
		}

		// create the instance
		err = modular.Initialize(ctx, ball, component.Selector)
		require.NoError(t, err)
	}

	eg := &errgroup.Group{}

	// start components
	for _, service := range re.Services {
		err := mud.ForEachDependency(service.Ball, service.Selector, func(component *mud.Component) error {
			return errs2.IgnoreCanceled(component.Run(ctx, eg))
		}, mud.All)
		require.NoError(t, err)
	}
	defer func() {
		for _, service := range re.Services {
			err := mud.ForEachDependencyReverse(service.Ball, service.Selector, func(component *mud.Component) error {
				return errs2.IgnoreCanceled(component.Close(ctx))
			}, mud.All)
			require.NoError(t, err)
		}
	}()
	callback(t, ctx, re)
	cancel()
	err := eg.Wait()
	require.NoError(t, errs2.IgnoreCanceled(err))
}

// initAndExec executes the given hook with (early) initialized parameters.
func initAndExec(ctx context.Context, ball *mud.Ball, hook any) error {
	val := reflect.ValueOf(hook)
	if val.Kind() != reflect.Func {
		return fmt.Errorf("hook must be a function")
	}

	ft := val.Type()
	for i := 0; i < ft.NumIn(); i++ {
		paramType := ft.In(i)

		if paramType.String() == "context.Context" || paramType == reflect.TypeOf(ball) {
			continue
		}
		// Look up the component from the ball
		component, found := mud.LookupByType(ball, paramType)
		if !found {
			return errs.New("dependency not found for hook parameter %d: %v", i, paramType)
		}

		err := component.Init(ctx)
		if err != nil {
			return errs.Wrap(err)
		}

	}

	return mud.Execute0(ctx, ball, hook)
}

// FindFirst finds the first component of the given type in the runtime environment.
// It searches for a component with the specified name and index, returning it if found.
// If not found, the test will fail with an appropriate error message.
func FindFirst[T any](t *testing.T, run RuntimeEnvironment, name string, ix int) T {
	for _, service := range run.Services {
		if service.Name == name && service.Index == ix {
			return mud.MustLookup[T](service.Ball)
		}
	}
	var ret T
	require.Fail(t, fmt.Sprintf("Component could not be found: %T", ret))
	return ret
}
