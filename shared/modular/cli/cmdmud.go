// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"
	"golang.org/x/exp/slices"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// MudCommand is a command that initializes and runs modular components.
type MudCommand struct {
	ball           *mud.Ball
	selector       mud.ComponentSelector // selector for components to initialize and run
	runSelector    mud.ComponentSelector // optional selector for components to run. Used for config list, where everything is used to initialize, but only the subcommand is executed.
	cfg            *ConfigSupport
	componentDebug string
}

// Setup implements clingy setup phase.
func (m *MudCommand) Setup(params clingy.Parameters) {
	ctx := context.Background()

	selectorStr := params.Flag("components", "Modular component selection. If empty, all default components will be running", "").(string)

	if m.selector == nil {
		m.selector = modular.CreateSelectorFromString(m.ball, selectorStr)
	} else if selectorStr != "" {
		m.selector = mud.Or(m.selector, modular.CreateSelectorFromString(m.ball, selectorStr))
	}

	m.componentDebug = params.Flag("debug-components", "Debug which components supposed to be run (selected or all)", "").(string)

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

// sortComponentNames is a helper function to sort component names, ignoring any leading '*' character.
func sortComponentNames(a, b string) int {
	a = strings.TrimPrefix(a, "*")
	b = strings.TrimPrefix(b, "*")
	return strings.Compare(a, b)
}

// Execute is the clingy entry point.
func (m *MudCommand) Execute(ctx context.Context) error {
	if m.componentDebug != "" {
		return m.printComponentInformation()
	}
	if m.runSelector == nil {
		m.runSelector = m.selector
	}
	err := mud.ForEachDependency(m.ball, m.runSelector, func(component *mud.Component) error {
		return component.Init(ctx)
	}, mud.All)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		shutdownTimeout := 15 * time.Second
		if timeoutStr := os.Getenv("STORJ_SHUTDOWN_TIMEOUT"); timeoutStr != "" {
			if timeoutSecs, parseErr := strconv.Atoi(timeoutStr); parseErr == nil && timeoutSecs > 0 {
				shutdownTimeout = time.Duration(timeoutSecs) * time.Second
			}
		}

		closeCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- mud.ForEachDependencyReverse(m.ball, m.runSelector, func(component *mud.Component) error {
				return component.Close(closeCtx)
			}, mud.All)
		}()

		select {
		case err = <-done:
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
			}
		case <-time.After(shutdownTimeout):
			if debugPath := os.Getenv("STORJ_SHUTDOWN_DEBUG_PATH"); debugPath != "" {
				pid := os.Getpid()
				timestamp := time.Now().Unix()
				filename := fmt.Sprintf("%d-%d.goroutines", pid, timestamp)
				fullPath := filepath.Join(debugPath, filename)

				buf := make([]byte, 1<<20) // 1MB buffer
				stackSize := runtime.Stack(buf, true)
				err := os.WriteFile(fullPath, buf[:stackSize], 0644)
				if err != nil {
					_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
				}
			}
			cancel()
			err = <-done
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "%v\n", err)
			}
		}
	}()

	eg, childCtx := errgroup.WithContext(ctx)
	err = mud.ForEachDependency(m.ball, m.runSelector, func(component *mud.Component) error {
		return component.Run(pprof.WithLabels(childCtx, pprof.Labels("component", component.Name())), eg)
	}, mud.All)
	if err != nil {
		return errs.Wrap(err)
	}

	return eg.Wait()
}

func (m *MudCommand) printComponentInformation() error {
	switch m.componentDebug {
	case "all":
		fmt.Println("All possible components to select from:")
		fmt.Println()
		var res []string
		for _, comp := range mud.Find(m.ball, mud.All) {
			res = append(res, comp.Name())
		}
		slices.SortFunc(res, sortComponentNames)
		for _, name := range res {
			fmt.Println(name)
		}
	case "selected":
		fmt.Println("The selected components which will be started/used:")
		fmt.Println()
		var res []string
		err := mud.ForEachDependency(m.ball, m.selector, func(component *mud.Component) error {
			res = append(res, component.Name())
			return nil
		}, mud.All)
		if err != nil {
			return errs.Wrap(err)
		}
		slices.SortFunc(res, sortComponentNames)
		for _, name := range res {
			fmt.Println(name)
		}
	default:
		return errs.New("Use `selected` or `all` as parameter for --debug-components")
	}
	return nil
}
