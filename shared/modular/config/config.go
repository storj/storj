// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package config

import (
	"context"

	"github.com/spf13/cobra"

	"storj.io/common/cfgstruct"
	"storj.io/common/process"
	"storj.io/storj/shared/mud"
)

// Config is a mud annotation.
// Structs annotated with this, will be exposed as flags / configs (with the given prefix).
type Config struct {
	Prefix string
}

// String implements String.
func (c Config) String() string {
	return "config prefix=" + c.Prefix
}

// RegisterConfig registers a config to mud, with all the information required for binding.
func RegisterConfig[T any](ball *mud.Ball, prefix string) {

	// we need a pointer to make it possible to bind
	mud.Provide[*T](ball, func() *T {
		var config T
		return &config
	})
	mud.Tag[*T, Config](ball, Config{Prefix: prefix})

	// helper, as many components require real instance.
	mud.View[*T, T](ball, func(t *T) T {
		return *t
	})
}

// BindAll will register all the Config annotated components as flags / config keys.
func BindAll(ctx context.Context, cmd *cobra.Command, ball *mud.Ball, selector mud.ComponentSelector, opts ...cfgstruct.BindOpt) error {
	return mud.ForEachDependency(ball, selector, func(component *mud.Component) error {
		return Bind(ctx, cmd, component, opts...)
	}, mud.Tagged[Config]())
}

// Bind binds the config for a command.
func Bind(ctx context.Context, cmd *cobra.Command, component *mud.Component, opts ...cfgstruct.BindOpt) error {
	err := component.Init(ctx)
	if err != nil {
		return err
	}
	// TODO call Verify for configs which implements it
	tag, found := mud.GetTagOf[Config](component)
	if !found {
		return nil
	}
	process.Bind(cmd, component.Instance(), append(opts, cfgstruct.Prefix(tag.Prefix))...)
	return nil
}
