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
	// RunWrapper is used to wrap the execution of test. Can be useful to run DB test. module will be added to all components.
	RunWrapper func(t *testing.T, fn func(t *testing.T, module func(*mud.Ball)))

	// Components is a list of components to run in the test environment (microservices like storagenode, satellite, etc.).
	Components []Component
}

// Modules is a list of modules that can be applied to a component.
type Modules []func(ball *mud.Ball)

// Customization defines how a component component should be created and run.
type Customization struct {
	// Modules define how the types can be created.
	Modules Modules

	// Selector defines which components to initialize and run.
	Selector mud.ComponentSelector

	// PreInit is a list of hooks that are executed before the component is initialized. Parameters will be injected by type.
	// This is called after the configuration is created, before the components are created. use func(c *ConfigType) to modify the config.
	PreInit []any
}

// Component is a configuration for a single component in the test environment.
type Component struct {
	Customization
	Name string
}

// NewComponent creates a new component with the given name and customizations.
func NewComponent(name string, customizations ...Customization) Component {
	c := Component{
		Name: name,
	}
	for _, customization := range customizations {
		c.Modules = append(c.Modules, customization.Modules...)

		if customization.Selector != nil {
			if c.Selector == nil {
				c.Selector = customization.Selector
			} else {
				c.Selector = mud.And(c.Selector, customization.Selector)
			}
		}

		c.PreInit = append(c.PreInit, customization.PreInit...)
	}
	return c
}

// WithModule adds a module to the component.
func WithModule(modules ...func(ball *mud.Ball)) Customization {
	return Customization{
		Modules: modules,
	}
}

// WithConfig can make it possible customize a config (use pointer).
func WithConfig[T any](fn func(T)) Customization {
	return Customization{
		PreInit: []any{fn},
	}
}

// WithRunningAll sets the component selector for the component.
func WithRunningAll(selector ...mud.ComponentSelector) Customization {
	return Customization{
		Selector: mud.Or(selector...),
	}
}

// WithRunning requests to Run T during the test.
func WithRunning[T any]() Customization {
	return Customization{
		Selector: mud.SelectIfExists[T](),
	}
}

// WithSelector sets a custom selector for the component.
func WithSelector(selector mud.ComponentSelector) Customization {
	return Customization{
		Selector: selector,
	}
}

// RuntimeEnvironment is the runtime environment of the test.
type RuntimeEnvironment struct {
	Services []Microservice
}

// Microservice is a single instance of a component in the test environment.
type Microservice struct {
	Name       string
	Index      int
	Ball       *mud.Ball
	Selector   mud.ComponentSelector
	WorkDir    string
	RunWrapper func(t *testing.T, fn func(t *testing.T))
}

// Run sets up and executes a test environment with the specified components and configuration.
// It initializes all components, executes pre-init hooks, starts all services,
// calls the provided callback function, and ensures proper cleanup afterward.
func Run(t *testing.T, c Config, callback func(t *testing.T, ctx context.Context, run RuntimeEnvironment)) {
	logger := zaptest.NewLogger(t)
	wrapper := func(t *testing.T, fn func(t *testing.T, module func(*mud.Ball))) {
		fn(t, nil)
	}
	if c.RunWrapper != nil {
		wrapper = c.RunWrapper
	}
	wrapper(t, func(t *testing.T, module func(*mud.Ball)) {
		tctx := testcontext.New(t)

		ctx, cancel := context.WithCancel(tctx)

		re := RuntimeEnvironment{}
		for ix, component := range c.Components {
			ball := mud.NewBall()

			if module != nil {
				module(ball)
			}

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
	})
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
