// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package mudplanet

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"

	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

// InitConfigDefaults initializes the configuration with the default values from the annotations.
func InitConfigDefaults(ctx context.Context, t *testing.T, ball *mud.Ball, selector mud.ComponentSelector, workDir string) error {
	return mud.ForEachDependency(ball, selector, func(component *mud.Component) error {
		if component.Instance() == nil {
			err := component.Init(ctx)
			require.NoError(t, err)
		}
		cfg := component.Instance()
		val := reflect.ValueOf(cfg).Elem()
		injectDefault(t, val, workDir)
		return nil
	}, mud.Tagged[config.Config]())
}

// injectDefault injects default values into the configuration struct from the annotation.
func injectDefault(t *testing.T, val reflect.Value, workDir string) {
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
			require.Fail(t, fmt.Sprintf("cannot get interface of field %s in %s", typeField.Name, val.Type()))
		}
		fieldaddr := fieldref.Interface()
		if fieldvalue, ok := fieldaddr.(pflag.Value); ok {
			strval := fmt.Sprintf("%v", defaultValue)
			err := fieldvalue.Set(strval)
			require.NoError(t, err, "Error on setting field %v\\%s with value %s", val.Type(), fieldName, strval)
			continue
		}

		if field.CanSet() {
			if field.Kind() == reflect.Struct {
				sub := reflect.New(field.Type()).Elem()
				injectDefault(t, sub, workDir)
				field.Set(sub)
				continue
			}

			if defaultValue == "" {
				continue
			}
			switch field.Kind() {
			case reflect.String:
				defaultValue = strings.ReplaceAll(defaultValue, "$HOST", "127.0.0.1")
				defaultValue = strings.ReplaceAll(defaultValue, "$CONFDIR", workDir)
				defaultValue = strings.ReplaceAll(defaultValue, "${CONFDIR}", workDir)
				field.SetString(defaultValue)
			case reflect.TypeOf(time.Duration(1)).Kind():
				duration, err := time.ParseDuration(defaultValue)
				require.NoError(t, err)
				field.Set(reflect.ValueOf(duration))
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				iv, err := strconv.Atoi(defaultValue)
				require.NoError(t, err)
				field.Set(reflect.ValueOf(iv))
			case reflect.Bool:
				if strings.ToLower(defaultValue) == "true" {
					field.SetBool(true)
				}
			case reflect.Float64:
				float, err := strconv.ParseFloat(defaultValue, 64)
				require.NoError(t, err)
				field.SetFloat(float)

			default:
				require.Fail(t, fmt.Sprintf("Unsupported type for default configuration value injection: %s (for %v)", field.Kind(), val.Type()))
			}
		}
	}
}
