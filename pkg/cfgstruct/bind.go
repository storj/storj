// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cfgstruct

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"

	"storj.io/storj/internal/memory"
)

// BindOpt is an option for the Bind method
type BindOpt func(vars map[string]confVar)

// ConfDir sets variables for default options called $CONFDIR and $CONFNAME.
func ConfDir(path string) BindOpt {
	val := filepath.Clean(os.ExpandEnv(path))
	return BindOpt(func(vars map[string]confVar) {
		vars["CONFDIR"] = confVar{val: val, nested: false}
		vars["CONFNAME"] = confVar{val: val, nested: false}
	})
}

// IdentityDir sets a variable for the default option called $IDENTITYDIR.
func IdentityDir(path string) BindOpt {
	val := filepath.Clean(os.ExpandEnv(path))
	return BindOpt(func(vars map[string]confVar) {
		vars["IDENTITYDIR"] = confVar{val: val, nested: false}
	})
}

// ConfDirNested sets variables for default options called $CONFDIR and $CONFNAME.
// ConfDirNested also appends the parent struct field name to the paths before
// descending into substructs.
func ConfDirNested(confdir string) BindOpt {
	val := filepath.Clean(os.ExpandEnv(confdir))
	return BindOpt(func(vars map[string]confVar) {
		vars["CONFDIR"] = confVar{val: val, nested: true}
		vars["CONFNAME"] = confVar{val: val, nested: true}
	})
}

type confVar struct {
	val    string
	nested bool
}

// Bind sets flags on a FlagSet that match the configuration struct
// 'config'. This works by traversing the config struct using the 'reflect'
// package. Will ignore fields with `setup:"true"` tag.
func Bind(flags FlagSet, config interface{}, opts ...BindOpt) {
	bind(flags, config, false, opts...)
}

// BindSetup sets flags on a FlagSet that match the configuration struct
// 'config'. This works by traversing the config struct using the 'reflect'
// package.
func BindSetup(flags FlagSet, config interface{}, opts ...BindOpt) {
	bind(flags, config, true, opts...)
}

func bind(flags FlagSet, config interface{}, setupCommand bool, opts ...BindOpt) {
	ptrtype := reflect.TypeOf(config)
	if ptrtype.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("invalid config type: %#v. Expecting pointer to struct.", config))
	}
	vars := map[string]confVar{}
	for _, opt := range opts {
		opt(vars)
	}
	bindConfig(flags, "", reflect.ValueOf(config).Elem(), vars, setupCommand, false)
}

func bindConfig(flags FlagSet, prefix string, val reflect.Value, vars map[string]confVar, setupCommand, setupStruct bool) {
	if val.Kind() != reflect.Struct {
		panic(fmt.Sprintf("invalid config type: %#v. Expecting struct.", val.Interface()))
	}
	typ := val.Type()

	resolvedVars := make(map[string]string, len(vars))
	{
		structpath := strings.Replace(prefix, ".", "/", -1)
		for k, v := range vars {
			if !v.nested {
				resolvedVars[k] = v.val
				continue
			}
			resolvedVars[k] = filepath.Join(v.val, structpath)
		}
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldval := val.Field(i)
		flagname := prefix + hyphenate(snakeCase(field.Name))
		if field.Tag.Get("internal") == "true" {
			continue
		}

		onlyForSetup := (field.Tag.Get("setup") == "true") || setupStruct
		// ignore setup params for non setup commands
		if !setupCommand && onlyForSetup {
			continue
		}

		switch field.Type.Kind() {
		case reflect.Struct:
			if field.Anonymous {
				bindConfig(flags, prefix, fieldval, vars, setupCommand, onlyForSetup)
			} else {
				bindConfig(flags, flagname+".", fieldval, vars, setupCommand, onlyForSetup)
			}
		case reflect.Array, reflect.Slice:
			digits := len(fmt.Sprint(fieldval.Len()))
			for j := 0; j < fieldval.Len(); j++ {
				padding := strings.Repeat("0", digits-len(fmt.Sprint(j)))
				bindConfig(flags, fmt.Sprintf("%s.%s%d.", flagname, padding, j), fieldval.Index(j), vars, setupCommand, onlyForSetup)
			}
		default:
			help := field.Tag.Get("help")
			def := field.Tag.Get("default")
			fieldaddr := fieldval.Addr().Interface()
			check := func(err error) {
				if err != nil {
					panic(fmt.Sprintf("invalid default value for %s: %#v", flagname, def))
				}
			}
			switch field.Type {
			case reflect.TypeOf(memory.Size(0)):
				check(fieldaddr.(*memory.Size).Set(def))
				flags.Var(fieldaddr.(*memory.Size), flagname, help)
			case reflect.TypeOf(int(0)):
				val, err := strconv.ParseInt(def, 0, strconv.IntSize)
				check(err)
				flags.IntVar(fieldaddr.(*int), flagname, int(val), help)
			case reflect.TypeOf(int64(0)):
				val, err := strconv.ParseInt(def, 0, 64)
				check(err)
				flags.Int64Var(fieldaddr.(*int64), flagname, val, help)
			case reflect.TypeOf(uint(0)):
				val, err := strconv.ParseUint(def, 0, strconv.IntSize)
				check(err)
				flags.UintVar(fieldaddr.(*uint), flagname, uint(val), help)
			case reflect.TypeOf(uint64(0)):
				val, err := strconv.ParseUint(def, 0, 64)
				check(err)
				flags.Uint64Var(fieldaddr.(*uint64), flagname, val, help)
			case reflect.TypeOf(time.Duration(0)):
				val, err := time.ParseDuration(def)
				check(err)
				flags.DurationVar(fieldaddr.(*time.Duration), flagname, val, help)
			case reflect.TypeOf(float64(0)):
				val, err := strconv.ParseFloat(def, 64)
				check(err)
				flags.Float64Var(fieldaddr.(*float64), flagname, val, help)
			case reflect.TypeOf(string("")):
				flags.StringVar(
					fieldaddr.(*string), flagname, expand(resolvedVars, def), help)
			case reflect.TypeOf(bool(false)):
				val, err := strconv.ParseBool(def)
				check(err)
				flags.BoolVar(fieldaddr.(*bool), flagname, val, help)
			default:
				panic(fmt.Sprintf("invalid field type: %s", field.Type.String()))
			}
			if onlyForSetup {
				setBoolAnnotation(flags, flagname, "setup")
			}
			if field.Tag.Get("user") == "true" {
				setBoolAnnotation(flags, flagname, "user")
			}
		}
	}
}

func setBoolAnnotation(flagset interface{}, name, key string) {
	flags, ok := flagset.(*pflag.FlagSet)
	if !ok {
		return
	}

	err := flags.SetAnnotation(name, key, []string{"true"})
	if err != nil {
		panic(fmt.Sprintf("unable to set %s annotation for %s: %v", key, name, err))
	}
}

func expand(vars map[string]string, val string) string {
	return os.Expand(val, func(key string) string { return vars[key] })
}

// FindConfigDirParam returns '--config-dir' param from os.Args (if exists)
func FindConfigDirParam() string {
	return FindFlagEarly("config-dir")
}

// FindIdentityDirParam returns '--identity-dir' param from os.Args (if exists)
func FindIdentityDirParam() string {
	return FindFlagEarly("identity-dir")
}

// FindFlagEarly retrieves the value of a flag before `flag.Parse` has been called
func FindFlagEarly(flagName string) string {
	// workaround to have early access to 'dir' param
	for i, arg := range os.Args {
		if strings.HasPrefix(arg, fmt.Sprintf("--%s=", flagName)) {
			return strings.TrimPrefix(arg, fmt.Sprintf("--%s=", flagName))
		} else if arg == fmt.Sprintf("--%s", flagName) && i < len(os.Args)-1 {
			return os.Args[i+1]
		}
	}
	return ""
}
