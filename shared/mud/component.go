// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mud

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

// StageName is the unique identifier of the stages (~lifecycle events).
type StageName string

// Component manages the lifecycle of a singleton Golang struct.
type Component struct {
	target reflect.Type

	instance any

	// Requirements are other components which is used by this component.
	// All requirements will be initialized/started before creating/running the component.
	requirements []reflect.Type

	create *Stage

	run *Stage

	close *Stage

	tags []any

	definition string
}

// Name returns with the human friendly name of the component.
func (c *Component) Name() string {
	return c.target.String()
}

// ID is the unque identifier of the component.
func (c *Component) ID() string {
	return fullyQualifiedTypeName(c.target)
}

// Init initializes the internal singleton instance.
func (c *Component) Init(ctx context.Context) error {
	if c.instance != nil {
		return nil
	}
	c.create.started = time.Now()
	err := c.create.run(nil, ctx)
	c.create.finished = time.Now()
	return err
}

// Run executes the Run stage function.
func (c *Component) Run(ctx context.Context, eg *errgroup.Group) error {
	if c.run == nil || !c.run.started.IsZero() {
		return nil
	}
	if c.instance == nil {
		return nil
	}

	if c.run.background {
		eg.Go(func() error {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "Panic in component: %s", c.ID())
					panic(r)
				}
			}()
			c.run.started = time.Now()
			err := c.run.run(c.instance, ctx)
			c.run.finished = time.Now()
			return err
		})
		return nil
	} else {
		c.run.started = time.Now()
		err := c.run.run(c.instance, ctx)
		c.run.finished = time.Now()
		return err
	}
}

// Close calls the Close stage function.
func (c *Component) Close(ctx context.Context) error {
	if c.close == nil || c.close.run == nil || !c.close.started.IsZero() || c.instance == nil {
		return nil
	}
	c.close.started = time.Now()
	err := c.close.run(c.instance, ctx)
	c.close.finished = time.Now()
	return err
}

// String returns with a string representation of the component.
func (c *Component) String() string {
	out := c.target.String()
	out += stageStr(c.create, "i")
	out += stageStr(c.run, "r")
	out += stageStr(c.close, "c")
	return out
}

func stageStr(stage *Stage, s string) string {
	if stage == nil {
		return "_"
	}
	if stage.started.IsZero() {
		return strings.ToLower(s)
	}
	return strings.ToUpper(s)
}

// AddRequirement marks the argument type as dependency of this component.
func (c *Component) AddRequirement(in reflect.Type) {
	if slices.Contains(c.requirements, in) {
		return
	}
	c.requirements = append(c.requirements, in)
}

// Instance returns the singleton instance of the component. Can be null, if not yet initialized.
func (c *Component) Instance() any {
	return c.instance
}

// GetTarget returns with type, which is used as n identifier in mud.
func (c *Component) GetTarget() reflect.Type {
	return c.target
}

// Stage represents a function which should be called on the component at the right time (like start, stop, init).
type Stage struct {
	run func(any, context.Context) error

	// should be executed in the background or not.
	background bool

	started time.Time

	finished time.Time
}
