// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cfgstruct

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// BindOpt is an option for the Bind method
type BindOpt func(vars map[string]confVar)

// ConfDirUnnested sets variables for default options called $CONFDIR and
// $CONFNAME. ConfDirUnnested does not append parent struct field names to the
// path when descending into substructs.
func ConfDirUnnested(confdir string) BindOpt {
	val := filepath.Clean(os.ExpandEnv(confdir))
	return BindOpt(func(vars map[string]confVar) {
		vars["CONFDIR"] = confVar{val: val, nested: false}
		vars["CONFNAME"] = confVar{val: val, nested: false}
	})
}

// ConfDir sets variables for default options called $CONFDIR and $CONFNAME.
// ConfDir also appends the parent struct field name to the paths before
// descending into substructs.
func ConfDir(confdir string) BindOpt {
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
// package.
func Bind(flags FlagSet, config interface{}, opts ...BindOpt) {
	ptrtype := reflect.TypeOf(config)
	if ptrtype.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("invalid config type: %#v. "+
			"Expecting pointer to struct.", config))
	}
	vars := map[string]confVar{}
	for _, opt := range opts {
		opt(vars)
	}
	bindConfig(flags, "", reflect.ValueOf(config).Elem(), vars)
}

var (
	whitespace = regexp.MustCompile(`\s+`)
)

func bindConfig(flags FlagSet, prefix string, val reflect.Value,
	vars map[string]confVar) {
	if val.Kind() != reflect.Struct {
		panic(fmt.Sprintf("invalid config type: %#v. Expecting struct.",
			val.Interface()))
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

		switch field.Type.Kind() {
		case reflect.Struct:
			if field.Anonymous {
				bindConfig(flags, prefix, fieldval, vars)
			} else {
				bindConfig(flags, flagname+".", fieldval, vars)
			}
		case reflect.Array, reflect.Slice:
			digits := len(fmt.Sprint(fieldval.Len()))
			for j := 0; j < fieldval.Len(); j++ {
				padding := strings.Repeat("0", digits-len(fmt.Sprint(j)))
				bindConfig(flags, fmt.Sprintf("%s.%s%d.", flagname, padding, j),
					fieldval.Index(j), vars)
			}
		default:
			tag := reflect.StructTag(
				whitespace.ReplaceAllString(string(field.Tag), " "))
			help := tag.Get("help")
			def := tag.Get("default")
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
			default:
				panic(fmt.Sprintf("invalid field type: %s", field.Type.String()))
			}
		}
	}
}

func expand(vars map[string]string, val string) string {
	return os.Expand(val, func(key string) string { return vars[key] })
}
