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

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"storj.io/storj/internal/version"
)

// BindOpt is an option for the Bind method
type BindOpt struct {
	isDev   *bool
	isSetup *bool
	varfn   func(vars map[string]confVar)
}

// ConfDir sets variables for default options called $CONFDIR and $CONFNAME.
func ConfDir(path string) BindOpt {
	val := filepath.Clean(os.ExpandEnv(path))
	return BindOpt{varfn: func(vars map[string]confVar) {
		vars["CONFDIR"] = confVar{val: val, nested: false}
	}}
}

// IdentityDir sets a variable for the default option called $IDENTITYDIR.
func IdentityDir(path string) BindOpt {
	val := filepath.Clean(os.ExpandEnv(path))
	return BindOpt{varfn: func(vars map[string]confVar) {
		vars["IDENTITYDIR"] = confVar{val: val, nested: false}
	}}
}

// SetupMode issues the bind in a mode where it does not ignore fields with the
// `setup:"true"` tag.
func SetupMode() BindOpt {
	setup := true
	return BindOpt{isSetup: &setup}
}

// UseDevDefaults forces the bind call to use development defaults unless
// UseReleaseDefaults is provided as a subsequent option.
// Without either, Bind will default to determining which defaults to use
// based on version.Build.Release
func UseDevDefaults() BindOpt {
	dev := true
	return BindOpt{isDev: &dev}
}

// UseReleaseDefaults forces the bind call to use release defaults unless
// UseDevDefaults is provided as a subsequent option.
// Without either, Bind will default to determining which defaults to use
// based on version.Build.Release
func UseReleaseDefaults() BindOpt {
	dev := false
	return BindOpt{isDev: &dev}
}

type confVar struct {
	val    string
	nested bool
}

// Bind sets flags on a FlagSet that match the configuration struct
// 'config'. This works by traversing the config struct using the 'reflect'
// package.
func Bind(flags FlagSet, config interface{}, opts ...BindOpt) {
	bind(flags, config, opts...)
}

func bind(flags FlagSet, config interface{}, opts ...BindOpt) {
	ptrtype := reflect.TypeOf(config)
	if ptrtype.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("invalid config type: %#v. Expecting pointer to struct.", config))
	}
	isDev := !version.Build.Release
	setupCommand := false
	vars := map[string]confVar{}
	for _, opt := range opts {
		if opt.varfn != nil {
			opt.varfn(vars)
		}
		if opt.isDev != nil {
			isDev = *opt.isDev
		}
		if opt.isSetup != nil {
			setupCommand = *opt.isSetup
		}
	}

	bindConfig(flags, "", reflect.ValueOf(config).Elem(), vars, setupCommand, false, isDev)
}

func bindConfig(flags FlagSet, prefix string, val reflect.Value, vars map[string]confVar, setupCommand, setupStruct bool, isDev bool) {
	if val.Kind() != reflect.Struct {
		panic(fmt.Sprintf("invalid config type: %#v. Expecting struct.", val.Interface()))
	}
	typ := val.Type()
	resolvedVars := make(map[string]string, len(vars))
	{
		structpath := strings.Replace(prefix, ".", string(filepath.Separator), -1)
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
		flagname := hyphenate(snakeCase(field.Name))

		if field.Tag.Get("noprefix") != "true" {
			flagname = prefix + flagname
		}

		if field.Tag.Get("internal") == "true" {
			continue
		}

		onlyForSetup := (field.Tag.Get("setup") == "true") || setupStruct
		// ignore setup params for non setup commands
		if !setupCommand && onlyForSetup {
			continue
		}

		fieldaddr := fieldval.Addr().Interface()
		if fieldvalue, ok := fieldaddr.(pflag.Value); ok {
			help := field.Tag.Get("help")
			var def string
			if isDev {
				def = getDefault(field.Tag, "devDefault", "releaseDefault", "default", flagname)
			} else {
				def = getDefault(field.Tag, "releaseDefault", "devDefault", "default", flagname)
			}

			err := fieldvalue.Set(def)
			if err != nil {
				panic(fmt.Sprintf("invalid default value for %s: %#v, %v", flagname, def, err))
			}
			flags.Var(fieldvalue, flagname, help)

			if onlyForSetup {
				setBoolAnnotation(flags, flagname, "setup")
			}
			if field.Tag.Get("user") == "true" {
				setBoolAnnotation(flags, flagname, "user")
			}
			if field.Tag.Get("hidden") == "true" {
				err := flags.MarkHidden(flagname)
				if err != nil {
					panic(fmt.Sprintf("mark hidden failed %s: %v", flagname, err))
				}
			}
			continue
		}

		switch field.Type.Kind() {
		case reflect.Struct:
			if field.Anonymous {
				bindConfig(flags, prefix, fieldval, vars, setupCommand, onlyForSetup, isDev)
			} else {
				bindConfig(flags, flagname+".", fieldval, vars, setupCommand, onlyForSetup, isDev)
			}
		case reflect.Array:
			digits := len(fmt.Sprint(fieldval.Len()))
			for j := 0; j < fieldval.Len(); j++ {
				padding := strings.Repeat("0", digits-len(fmt.Sprint(j)))
				bindConfig(flags, fmt.Sprintf("%s.%s%d.", flagname, padding, j), fieldval.Index(j), vars, setupCommand, onlyForSetup, isDev)
			}
		default:
			help := field.Tag.Get("help")
			var def string
			if isDev {
				def = getDefault(field.Tag, "devDefault", "releaseDefault", "default", flagname)
			} else {
				def = getDefault(field.Tag, "releaseDefault", "devDefault", "default", flagname)
			}
			fieldaddr := fieldval.Addr().Interface()
			check := func(err error) {
				if err != nil {
					panic(fmt.Sprintf("invalid default value for %s: %#v", flagname, def))
				}
			}
			switch field.Type {
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
			case reflect.TypeOf([]string(nil)):
				flags.StringArrayVar(fieldaddr.(*[]string), flagname, nil, help)
			default:
				panic(fmt.Sprintf("invalid field type: %s", field.Type.String()))
			}
			if onlyForSetup {
				setBoolAnnotation(flags, flagname, "setup")
			}
			if field.Tag.Get("user") == "true" {
				setBoolAnnotation(flags, flagname, "user")
			}
			if field.Tag.Get("hidden") == "true" {
				err := flags.MarkHidden(flagname)
				check(err)
			}
		}
	}
}

func getDefault(tag reflect.StructTag, preferred, opposite, fallback, flagname string) string {
	if val, ok := tag.Lookup(preferred); ok {
		if _, oppositeExists := tag.Lookup(opposite); !oppositeExists {
			panic(fmt.Sprintf("%q defined but %q missing for %v", preferred, opposite, flagname))
		}
		if _, fallbackExists := tag.Lookup(fallback); fallbackExists {
			panic(fmt.Sprintf("%q defined along with %q fallback for %v", preferred, fallback, flagname))
		}
		return val
	}
	if _, oppositeExists := tag.Lookup(opposite); oppositeExists {
		panic(fmt.Sprintf("%q missing but %q defined for %v", preferred, opposite, flagname))
	}
	return tag.Get(fallback)
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

// FindDefaultsParam returns '--defaults' param from os.Args (if it exists)
func FindDefaultsParam() string {
	return FindFlagEarly("defaults")
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

// SetupFlag sets up flags that are needed before `flag.Parse` has been called
func SetupFlag(log *zap.Logger, cmd *cobra.Command, dest *string, name, value, usage string) {
	if foundValue := FindFlagEarly(name); foundValue != "" {
		value = foundValue
	}
	cmd.PersistentFlags().StringVar(dest, name, value, usage)
	if cmd.PersistentFlags().SetAnnotation(name, "setup", []string{"true"}) != nil {
		log.Sugar().Errorf("Failed to set 'setup' annotation for '%s'", name)
	}
}

// DefaultsType returns the type of defaults (release/dev) this binary should use
func DefaultsType() string {
	// define a flag so that the flag parsing system will be happy.
	defaults := strings.ToLower(FindDefaultsParam())
	if defaults != "" {
		return defaults
	}
	if version.Build.Release {
		return "release"
	}
	return "dev"
}

// DefaultsFlag sets up the defaults=dev/release flag options, which is needed
// before `flag.Parse` has been called
func DefaultsFlag(cmd *cobra.Command) BindOpt {
	// define a flag so that the flag parsing system will be happy.
	defaults := DefaultsType()

	// we're actually going to ignore this flag entirely and parse the commandline
	// arguments early instead
	_ = cmd.PersistentFlags().String("defaults", defaults,
		"determines which set of configuration defaults to use. can either be 'dev' or 'release'")

	switch defaults {
	case "dev":
		return UseDevDefaults()
	case "release":
		return UseReleaseDefaults()
	default:
		panic(fmt.Sprintf("unsupported defaults value %q", FindDefaultsParam()))
	}
}
