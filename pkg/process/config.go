// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"bytes"
	"flag"
	"sort"

	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
	yaml "gopkg.in/yaml.v2"

	"storj.io/common/fpath"
	"storj.io/storj/pkg/cfgstruct"
)

// SaveConfigOption is a function that updates the options for SaveConfig.
type SaveConfigOption func(*SaveConfigOptions)

// SaveConfigOptions controls the behavior of SaveConfig.
type SaveConfigOptions struct {
	Overrides        map[string]interface{}
	RemoveDeprecated bool
}

// SaveConfigWithOverrides sets the overrides to the provided map.
func SaveConfigWithOverrides(overrides map[string]interface{}) SaveConfigOption {
	return func(opts *SaveConfigOptions) {
		opts.Overrides = overrides
	}
}

// SaveConfigWithOverride adds a single override to SaveConfig.
func SaveConfigWithOverride(name string, value interface{}) SaveConfigOption {
	return func(opts *SaveConfigOptions) {
		if opts.Overrides == nil {
			opts.Overrides = make(map[string]interface{})
		}
		opts.Overrides[name] = value
	}
}

// SaveConfigRemovingDeprecated tells SaveConfig to not store deprecated flags.
func SaveConfigRemovingDeprecated() SaveConfigOption {
	return func(opts *SaveConfigOptions) {
		opts.RemoveDeprecated = true
	}
}

// SaveConfig will save only the user-specific flags with default values to
// outfile with specific values specified in 'overrides' overridden.
func SaveConfig(cmd *cobra.Command, outfile string, opts ...SaveConfigOption) error {
	// step 0. apply any options to change the behavior
	//

	var options SaveConfigOptions
	for _, opt := range opts {
		opt(&options)
	}

	// step 1. load all of the configuration settings we are going to save
	//

	flags := cmd.Flags()
	vip, err := Viper(cmd)
	if err != nil {
		return errs.Wrap(err)
	}
	if err := vip.MergeConfigMap(options.Overrides); err != nil {
		return errs.Wrap(err)
	}
	settings := vip.AllSettings()

	// step 2. construct some data describing what exactly we're saving to the
	// config file, and how they're saved.
	//

	type configValue struct {
		value   interface{}
		comment string
		set     bool
	}
	flat := make(map[string]configValue)
	flatKeys := make([]string, 0)

	// N.B. we have to pre-declare the function so that it can make recursive calls.
	var filterAndFlatten func(string, map[string]interface{})
	filterAndFlatten = func(base string, settings map[string]interface{}) {
		for key, value := range settings {
			if value, ok := value.(map[string]interface{}); ok {
				filterAndFlatten(base+key+".", value)
				continue
			}
			fullKey := base + key

			// the help key should not be persisted to the config file but
			// cannot use the FlagSource source annotation since it is a
			// standard flags and not pflags.
			// TODO: figure out a better way to do for standard flags than
			// hardcoding in pkg/process.
			if fullKey == "help" {
				continue
			}

			// gather information about the flag under consideration
			var (
				changed    bool
				setup      bool
				hidden     bool
				user       bool
				deprecated bool
				source     string
				comment    string
				typ        string

				_, overrideExists = options.Overrides[fullKey]
			)
			if f := flags.Lookup(fullKey); f != nil { // first check pflags
				// When a value is loaded from the file, the flag won't be
				// "changed" but we still need to persist it. Therefore, for
				// the following code, "changed" means that either a flag has
				// been explicitly set or a value that differs from the
				// default.
				changed = f.Changed || f.Value.String() != f.DefValue
				setup = readBoolAnnotation(f, "setup")
				hidden = readBoolAnnotation(f, "hidden")
				user = readBoolAnnotation(f, "user")
				deprecated = readBoolAnnotation(f, "deprecated")
				source = readSourceAnnotation(f)
				comment = f.Usage
				typ = f.Value.Type()
			} else if f := flag.Lookup(fullKey); f != nil { // then stdlib flags
				changed = f.Value.String() != f.DefValue
				comment = f.Usage
			} else {
				// by default we store config values we know nothing about. we
				// absue the meaning of "changed" to include this case.
				changed = true
			}

			// in any of these cases, don't store the key in the config file
			if setup ||
				hidden ||
				options.RemoveDeprecated && deprecated ||
				source == cfgstruct.FlagSource {
				continue
			}

			// viper is super cool and doesn't cast floats automatically, so we
			// handle that ourselves.
			if typ == "float64" {
				value = cast.ToFloat64(value)
			}

			flatKeys = append(flatKeys, fullKey)
			flat[fullKey] = configValue{
				value:   value,
				comment: comment,
				set:     user || changed || overrideExists,
			}
		}
	}
	filterAndFlatten("", settings)
	sort.Strings(flatKeys)

	// step 3. write out the configuration file
	//

	var nl = []byte("\n")
	var lines [][]byte
	for _, key := range flatKeys {
		config := flat[key]

		if config.comment != "" {
			lines = append(lines, []byte("# "+config.comment))
		}

		data, err := yaml.Marshal(map[string]interface{}{key: config.value})
		if err != nil {
			return errs.Wrap(err)
		}
		dataLines := bytes.Split(bytes.TrimSpace(data), nl)

		// if the config value is set, concat in the yaml lines
		if config.set {
			lines = append(lines, dataLines...)
		} else {
			// otherwise, add them in but commented out
			for _, line := range dataLines {
				lines = append(lines, append([]byte("# "), line...))
			}
		}

		// add a blank line separator
		lines = append(lines, nil)
	}

	return errs.Wrap(fpath.AtomicWriteFile(outfile, bytes.Join(lines, nl), 0600))
}

// readSourceAnnotation is a helper to return the source annotation or cfgstruct.AnySource if unset.
func readSourceAnnotation(flag *pflag.Flag) string {
	annotation := flag.Annotations["source"]
	if len(annotation) == 0 {
		return cfgstruct.AnySource
	}
	return annotation[0]
}

// readBoolAnnotation is a helper to see if a boolean annotation is set to true on the flag.
func readBoolAnnotation(flag *pflag.Flag, key string) bool {
	annotation := flag.Annotations[key]
	return len(annotation) > 0 && annotation[0] == "true"
}
