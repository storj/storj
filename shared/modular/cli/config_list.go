// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// ConfigList is a command that lists all configuration options for modular components.
type ConfigList struct {
	env  bool
	ball *mud.Ball
}

// NewConfigList creates a new ConfigList command.
func NewConfigList(ball *mud.Ball) *ConfigList {
	return &ConfigList{
		ball: ball,
	}
}

// Run executes the subcommand.
func (c *ConfigList) Run(ctx context.Context) error {
	return mud.ForEachDependency(c.ball, mud.All, func(component *mud.Component) error {
		ct, found := mud.GetTagOf[config.Config](component)
		if !found {
			return nil
		}
		if ct.Prefix == "" {
			return nil
		}
		fmt.Printf("# %s config options (%s):\n", component.Name(), ct.Prefix)
		fmt.Println()
		instance := component.Instance()
		if instance == nil {
			err := component.Init(ctx)
			if err != nil {
				return err
			}
		}
		envNameReplacer := strings.NewReplacer(".", "_", "-", "_")
		val := reflect.ValueOf(component.Instance())
		for i := 0; i < val.Elem().NumField(); i++ {
			field := val.Elem().Type().Field(i)
			key := ct.Prefix + "." + hyphenate(camelToSnakeCase(field.Name))
			if c.env {
				key = "STORJ_"
				envNameReplacer.Replace(strings.ToUpper(key))
			}
			help := field.Tag.Get("help")
			if help == "" {
				help = field.Tag.Get("usage")
			}
			fmt.Printf("  %s: %s -- %s\n", key, returnAsString(val.Elem().Field(i)), help)
		}
		fmt.Println()
		fmt.Println()
		return nil
	}, mud.Tagged[config.Config]())
}

func returnAsString(field reflect.Value) string {
	if field.Kind() == reflect.String {
		return field.String()
	} else if field.Kind() == reflect.Bool {
		if field.Bool() {
			return "true"
		}
		return "false"
	} else if field.Kind() == reflect.Int || field.Kind() == reflect.Int64 || field.Kind() == reflect.Int32 {
		return fmt.Sprintf("%d", field.Int())
	} else if field.Kind() == reflect.Float64 || field.Kind() == reflect.Float32 {
		return fmt.Sprintf("%f", field.Float())
	} else if field.Kind() == reflect.Slice || field.Kind() == reflect.Array {
		return fmt.Sprintf("%v", field.Interface())
	}
	return fmt.Sprintf("%v", field.Interface())
}
