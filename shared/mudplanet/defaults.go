// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package mudplanet

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// InitConfigDefaults initializes the configuration with the default values from the annotations.
func InitConfigDefaults(ctx context.Context, ball *mud.Ball, selector mud.ComponentSelector, workDir string) error {
	return mud.ForEachDependency(ball, selector, func(component *mud.Component) error {
		if component.Instance() == nil {
			if err := component.Init(ctx); err != nil {
				return err
			}
		}
		cfg := component.Instance()
		val := reflect.ValueOf(cfg).Elem()
		return injectDefault(val, workDir)
	}, mud.Tagged[config.Config]())
}

// injectDefault injects default values into the configuration struct from the annotation.
func injectDefault(val reflect.Value, workDir string) error {
	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		typeField := val.Type().Field(i)
		fieldName := typeField.Name

		defaultValue := ""
		var isSet bool
		for _, defaultType := range []string{"testDefault", "devDefault", "default", "releaseDefault"} {
			defaultValue, isSet = typeField.Tag.Lookup(defaultType)
			if isSet {
				break
			}
		}

		fieldval := val.Field(i)
		fieldref := fieldval.Addr()
		if !fieldref.CanInterface() {
			return fmt.Errorf("cannot get interface of field %s in %s", typeField.Name, val.Type())
		}
		fieldaddr := fieldref.Interface()
		if fieldvalue, ok := fieldaddr.(pflag.Value); ok {
			strval := fmt.Sprintf("%v", defaultValue)
			if err := fieldvalue.Set(strval); err != nil {
				return fmt.Errorf("error on setting field %v\\%s with value %s: %w", val.Type(), fieldName, strval, err)
			}
			continue
		}

		if field.CanSet() {
			if field.Kind() == reflect.Struct {
				sub := reflect.New(field.Type()).Elem()
				if err := injectDefault(sub, workDir); err != nil {
					return err
				}
				field.Set(sub)
				continue
			}

			if defaultValue == "" {
				continue
			}
			defaultValue = strings.ReplaceAll(defaultValue, "$HOST", "127.0.0.1")
			defaultValue = strings.ReplaceAll(defaultValue, "$CONFDIR", workDir)
			defaultValue = strings.ReplaceAll(defaultValue, "${CONFDIR}", workDir)
			defaultValue = strings.ReplaceAll(defaultValue, "$TESTINTERVAL", "30s")
			switch field.Kind() {
			case reflect.String:
				field.SetString(defaultValue)
			case reflect.Int:
				iv, err := strconv.Atoi(defaultValue)
				if err != nil {
					return fmt.Errorf("error in default configuration value injection for (%v).%v: %w", val.Type(), fieldName, err)
				}
				field.Set(reflect.ValueOf(iv))
			case reflect.Int32:
				iv, err := strconv.Atoi(defaultValue)
				if err != nil {
					return fmt.Errorf("error in default configuration value injection for (%v).%v: %w", val.Type(), fieldName, err)
				}
				field.Set(reflect.ValueOf(int32(iv)))
			case reflect.Int64:
				if field.Type() == reflect.TypeFor[time.Duration]() {
					duration, err := time.ParseDuration(defaultValue)
					if err != nil {
						return fmt.Errorf("error in default configuration value injection for (%v).%v: %w", val.Type(), fieldName, err)
					}
					field.Set(reflect.ValueOf(duration))
				} else {
					iv, err := strconv.Atoi(defaultValue)
					if err != nil {
						return fmt.Errorf("error in default configuration value injection for (%v).%v: %w", val.Type(), fieldName, err)
					}
					field.Set(reflect.ValueOf(int64(iv)))
				}
			case reflect.Uint:
				iv, err := strconv.Atoi(defaultValue)
				if err != nil {
					return fmt.Errorf("error in default configuration value injection for (%v).%v: %w", val.Type(), fieldName, err)
				}
				field.Set(reflect.ValueOf(uint(iv)))
			case reflect.Uint32:
				iv, err := strconv.Atoi(defaultValue)
				if err != nil {
					return fmt.Errorf("error in default configuration value injection for (%v).%v: %w", val.Type(), fieldName, err)
				}
				field.Set(reflect.ValueOf(uint32(iv)))
			case reflect.Uint64:
				iv, err := strconv.Atoi(defaultValue)
				if err != nil {
					return fmt.Errorf("error in default configuration value injection for (%v).%v: %w", val.Type(), fieldName, err)
				}
				field.Set(reflect.ValueOf(uint64(iv)))
			case reflect.Bool:
				if strings.ToLower(defaultValue) == "true" {
					field.SetBool(true)
				}
			case reflect.Float64:
				float, err := strconv.ParseFloat(defaultValue, 64)
				if err != nil {
					return fmt.Errorf("error in default configuration value injection for (%v).%v: %w", val.Type(), fieldName, err)
				}
				field.SetFloat(float)
			case reflect.Slice:
				switch field.Type().Elem().Kind() {
				case reflect.String:
					field.Set(reflect.ValueOf(strings.Split(defaultValue, ",")))
				default:
					return fmt.Errorf("unsupported slice type for default configuration value injection: %s (for %v)", field.Type().Elem().Kind(), val.Type())
				}
			default:
				return fmt.Errorf("unsupported type for default configuration value injection: %s (for %v)", field.Kind(), val.Type())
			}
		}
	}
	return nil
}
