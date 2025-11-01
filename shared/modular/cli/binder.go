// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/pflag"
	"github.com/zeebo/clingy"
)

// bindConfig binds required configuration parameters to clingy params.
// This is called during clingy Setup phase, register flags, and fills the default values.
func bindConfig(params clingy.Parameters, prefix string, refVal reflect.Value, cfg *ConfigSupport) {
	val := refVal.Elem()
	if val.Kind() != reflect.Struct {
		panic(fmt.Sprintf("invalid config type: %#v. Expecting struct.", val.Interface()))
	}
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}

		fieldval := val.Field(i)
		flagname := snakeToHyphenatedCase(field.Name)

		if field.Tag.Get("noprefix") != "true" && prefix != "" {
			flagname = prefix + "." + flagname
		}

		if !fieldval.CanAddr() {
			panic(fmt.Sprintf("cannot addr field %s in %s", field.Name, typ))
		}

		if field.Tag.Get("internal") == "true" {
			continue
		}

		if field.Tag.Get("noflag") == "true" {
			err := cfg.GetSubtree(flagname, fieldval.Addr().Interface())
			if err != nil {
				panic(err)
			}
			continue
		}

		fieldref := fieldval.Addr()

		help := field.Tag.Get("help")
		// TODO: dev and test modes are not supported here

		defaultValue := field.Tag.Get("releaseDefault")
		if defaultValue == "" {
			defaultValue = field.Tag.Get("default")
		}
		defaultValue = strings.ReplaceAll(defaultValue, "$IDENTITYDIR", cfg.identityDir)
		defaultValue = strings.ReplaceAll(defaultValue, "$CONFDIR", cfg.configDir)

		def := func() interface{} {
			if field.Tag.Get("required") == "true" {
				return clingy.Required
			} else {
				return defaultValue
			}
		}

		// the prefix for recursive calls
		pf := func(t string) string {
			var s []string
			if prefix != "" {
				s = append(s, prefix)
			}
			if t != "" {
				s = append(s, snakeToHyphenatedCase(t))
			}
			return strings.Join(s, ".")
		}

		// if it's a pflag.Value implementation, let's the interface handle it
		fieldaddr := fieldref.Interface()
		if pfv, ok := fieldaddr.(pflag.Value); ok {
			val := getFlagValue(params, flagname, help, def())
			if val == nil {
				continue
			}
			if strv, ok := val.(string); ok {
				err := pfv.Set(strv)
				if err != nil {
					panic(fmt.Sprintf("invalid value for %s: %v", flagname, err))
				}
				continue
			}
			panic(fmt.Sprintf("cannot set default value for %s/%s: %T, val:%T %v", refVal.Type(), flagname, pfv, val, val))
		}

		if pfv, ok := fieldval.Interface().(pflag.Value); ok {
			panic(fmt.Sprintf("cannot set default value for %s/%s: %T", refVal.Type(), flagname, pfv))
		}

		// if it's a struct, we need to recurse into it
		if field.Type.Kind() == reflect.Struct {
			if field.Anonymous {
				bindConfig(params, pf(""), fieldref, cfg)
			} else {
				bindConfig(params, pf(field.Name), fieldref, cfg)
			}
			continue
		}

		// same for struct pointers
		if field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.Struct {
			// Initialize the pointer if it's nil
			if fieldval.IsNil() {
				if !fieldval.CanSet() {
					panic(fmt.Sprintf("cannot set field %s in %s", field.Name, typ))
				}
				fieldval.Set(reflect.New(field.Type.Elem()))
			}
			if field.Anonymous {
				bindConfig(params, pf(""), fieldref.Elem(), cfg)
			} else {
				bindConfig(params, pf(field.Name), fieldref.Elem(), cfg)
			}
			continue
		}

		invalidConversionPanic := func(field reflect.StructField, val any) {
			panic(fmt.Sprintf("invalid field type conversion: %s %T->%s (%v)", field.Name, val, field.Type.String(), val))
		}
		// here is the binding, where we got the real value.
		// Now we can assume it's a simple value, and let's clingy just get the actual value.
		// Time to set it back to the config reference (flag.Value may already be handled earlier).
		val := getFlagValue(params, flagname, help, def())
		if val == nil {
			continue
		}

		switch field.Type {
		case reflect.TypeOf(0):
			switch v := val.(type) {
			case int:
				fieldval.SetInt(int64(v))
			case string:
				if v == "" {
					continue
				}
				nv, err := strconv.Atoi(v)
				if err != nil {
					panic(err)
				}
				fieldval.SetInt(int64(nv))

			default:
				invalidConversionPanic(field, val)
			}

		case reflect.TypeOf(int64(0)):
			switch v := val.(type) {
			case string:
				if v == "" {
					continue
				}
				nv, err := strconv.Atoi(v)
				if err != nil {
					invalidConversionPanic(field, val)
				}
				fieldval.SetInt(int64(nv))
			case int64:
				fieldval.SetInt(v)
			default:
				invalidConversionPanic(field, val)
			}
		case reflect.TypeOf(uint(0)), reflect.TypeOf(uint64(0)), reflect.TypeOf(uint32(0)):
			switch v := val.(type) {
			case string:
				if v == "" {
					continue
				}
				nv, err := strconv.Atoi(v)
				if err != nil {
					invalidConversionPanic(field, val)
				}
				fieldval.SetUint(uint64(nv))
			default:
				fieldval.SetUint(reflect.ValueOf(val).Uint())
			}
		case reflect.TypeOf(time.Duration(0)):
			val, err := time.ParseDuration(val.(string))
			if err != nil {
				panic(err)
			}
			fieldval.Set(reflect.ValueOf(val))
		case reflect.TypeOf(float64(0)):
			switch v := val.(type) {
			case float64:
				fieldval.SetFloat(v)
			case string:
				if v == "" {
					continue
				}
				pf, err := strconv.ParseFloat(v, 64)
				if err != nil {
					invalidConversionPanic(field, val)
				}
				fieldval.SetFloat(pf)
			default:
				invalidConversionPanic(field, val)
			}
		case reflect.TypeOf(""):
			fieldval.SetString(val.(string))
		case reflect.TypeOf(false):
			switch bc := val.(type) {
			case bool:
				fieldval.SetBool(bc)
			case string:
				if bc == "" {
					continue
				}
				rv, err := strconv.ParseBool(strings.TrimSpace(strings.ToLower(bc)))
				if err != nil {
					invalidConversionPanic(field, val)
				}
				fieldval.SetBool(rv)
			default:
				invalidConversionPanic(field, val)
			}

		case reflect.TypeOf([]string(nil)):
			switch bc := val.(type) {
			case string:
				if bc == "" {
					continue
				}
				fieldval.Set(reflect.ValueOf(strings.Split(bc, ",")))
			default:
				invalidConversionPanic(field, val)
			}
		default:
			invalidConversionPanic(field, val)
		}

	}
}

// replace defined the naming convention to get environment variable names.
var replacer = strings.NewReplacer(".", "_", "-", "_")

func getFlagValue(params clingy.Parameters, flagname string, help string, def interface{}) interface{} {
	result := params.Flag(flagname, help, def)
	// at this point we have registered flag + we have available value
	// but environment variable may override it
	prefix := "STORJ"
	if os.Getenv("STORJ_ENV_PREFIX") != "" {
		prefix = os.Getenv("STORJ_ENV_PREFIX")
	}
	envName := prefix + "_" + replacer.Replace(strings.ToUpper(flagname))
	if os.Getenv(envName) != "" {
		return os.Getenv(envName)
	}
	return result
}

func snakeToHyphenatedCase(val string) string {
	return hyphenate(camelToSnakeCase(val))
}

func hyphenate(val string) string {
	return strings.ReplaceAll(val, "_", "-")
}

func camelToSnakeCase(val string) string {
	// don't you think this function should be in the standard library?
	// seems useful
	if len(val) <= 1 {
		return strings.ToLower(val)
	}
	runes := []rune(val)
	rv := make([]rune, 0, len(runes))
	for i := 0; i < len(runes); i++ {
		rv = append(rv, unicode.ToLower(runes[i]))
		if i < len(runes)-1 &&
			unicode.IsLower(runes[i]) &&
			unicode.IsUpper(runes[i+1]) {
			// lower-to-uppercase case
			rv = append(rv, '_')
		} else if i < len(runes)-2 &&
			unicode.IsUpper(runes[i]) &&
			unicode.IsUpper(runes[i+1]) &&
			unicode.IsLower(runes[i+2]) {
			// end-of-acronym case
			rv = append(rv, '_')
		}
	}
	return string(rv)
}
