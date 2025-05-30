// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"runtime/pprof"
	"time"

	"github.com/pkg/errors"
	"github.com/zeebo/clingy"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// MudCommand is a command that initializes and runs modular components.
type MudCommand struct {
	ball     *mud.Ball
	selector mud.ComponentSelector
	cfg      *ConfigSupport
}

// Setup implements clingy setup phase.
func (m *MudCommand) Setup(params clingy.Parameters) {
	ctx := context.Background()
	if m.selector == nil {
		selectorStr := params.Flag("components", "Modular component selection. If empty, all default components will be running", "").(string)
		m.selector = modular.CreateSelectorFromString(m.ball, selectorStr)
	}

	// create all the config structs
	err := mud.ForEachDependency(m.ball, m.selector, func(component *mud.Component) error {
		return component.Init(ctx)
	}, mud.Tagged[config.Config]())
	if err != nil {
		panic(err)
	}

	// register config structs as clingy parameters
	err = mud.ForEachDependency(m.ball, m.selector, func(component *mud.Component) error {

		tag, found := mud.GetTagOf[config.Config](component)
		if !found {
			return nil
		}

		bindConfig(params, tag.Prefix, reflect.ValueOf(component.Instance()), m.cfg)
		return nil
	}, mud.Tagged[config.Config]())
	if err != nil {
		panic(err)
	}
}

// Execute is the clingy entry point.
func (m *MudCommand) Execute(ctx context.Context) error {
	err := mud.ForEachDependency(m.ball, m.selector, func(component *mud.Component) error {
		return component.Init(ctx)
	}, mud.All)
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		err = mud.ForEachDependencyReverse(m.ball, m.selector, func(component *mud.Component) error {
			return component.Close(closeCtx)
		}, mud.All)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
		}
	}()

	eg := &errgroup.Group{}
	err = mud.ForEachDependency(m.ball, m.selector, func(component *mud.Component) error {
		return component.Run(pprof.WithLabels(ctx, pprof.Labels("component", component.Name())), eg)
	}, mud.All)
	if err != nil {
		return errors.WithStack(err)
	}

	return eg.Wait()
}
