// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
	yaml "gopkg.in/yaml.v2"
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
	var options SaveConfigOptions
	for _, opt := range opts {
		opt(&options)
	}

	flags := cmd.Flags()
	vip, err := Viper(cmd)
	if err != nil {
		return errs.Wrap(err)
	}

	// merge in the overrides and grab the settings.
	if err := vip.MergeConfigMap(options.Overrides); err != nil {
		return errs.Wrap(err)
	}
	settings := vip.AllSettings()

	// filter any settings we shouldn't save due to flag metadata.
	var filterSettings func(string, map[string]interface{})
	filterSettings = func(base string, settings map[string]interface{}) {
		for key, value := range settings {
			if value, ok := value.(map[string]interface{}); ok {
				filterSettings(base+key+".", value)
				if len(value) == 0 {
					delete(settings, key)
				}
				continue
			}

			fullKey := base + key
			_, overrideExists := options.Overrides[fullKey]
			changed, setup, hidden, user, deprecated := false, false, false, false, false
			if f := flags.Lookup(fullKey); f != nil {
				changed = f.Changed
				setup = readBoolAnnotation(f, "setup")
				hidden = readBoolAnnotation(f, "hidden")
				user = readBoolAnnotation(f, "user")
				deprecated = readBoolAnnotation(f, "deprecated")
			} else if f := flag.Lookup(fullKey); f != nil {
				changed = f.Value.String() != f.DefValue
			} else {
				// by default we store config values we know nothing about
				continue
			}

			// in any of these cases, don't store the key in the file
			switch {
			case setup:
			case hidden:
			case !user && !changed && !overrideExists:
			case options.RemoveDeprecated && deprecated:

			default:
				continue
			}
			delete(settings, key)
		}
	}
	filterSettings("", settings)

	var data []byte
	if len(settings) > 0 {
		data, err = yaml.Marshal(settings)
		if err != nil {
			return errs.Wrap(err)
		}
	}
	return errs.Wrap(atomicWrite(outfile, 0600, data))
}

// readBoolAnnotation is a helper to see if a boolean annotation is set to true on the flag.
func readBoolAnnotation(flag *pflag.Flag, key string) bool {
	annotation := flag.Annotations[key]
	return len(annotation) > 0 && annotation[0] == "true"
}

// atomicWrite is a helper to atomically write the data to the outfile.
func atomicWrite(outfile string, mode os.FileMode, data []byte) (err error) {
	fh, err := ioutil.TempFile(filepath.Dir(outfile), filepath.Base(outfile))
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if err != nil {
			err = errs.Combine(err, fh.Close())
			err = errs.Combine(err, os.Remove(fh.Name()))
		}
	}()
	if _, err := fh.Write(data); err != nil {
		return errs.Wrap(err)
	}
	if err := fh.Sync(); err != nil {
		return errs.Wrap(err)
	}
	if err := fh.Close(); err != nil {
		return errs.Wrap(err)
	}
	if err := os.Rename(fh.Name(), outfile); err != nil {
		return errs.Wrap(err)
	}
	return nil
}
