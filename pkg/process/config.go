// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/zeebo/errs"
	yaml "gopkg.in/yaml.v2"
)

// SaveConfig will save only the user-specific flags with default values to
// outfile with specific values specified in 'overrides' overridden.
func SaveConfig(cmd *cobra.Command, outfile string, overrides map[string]interface{}) error {
	flags := cmd.Flags()
	vip, err := Viper(cmd)
	if err != nil {
		return errs.Wrap(err)
	}

	// merge in the overrides and grab the settings.
	if err := vip.MergeConfigMap(overrides); err != nil {
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
			_, overrideExists := overrides[fullKey]
			changed, setup, hidden, user := false, false, false, false
			if f := flags.Lookup(fullKey); f != nil {
				changed = f.Changed
				setup = readBoolAnnotation(f, "setup")
				hidden = readBoolAnnotation(f, "hidden")
				user = readBoolAnnotation(f, "user")
			} else if f := flag.Lookup(fullKey); f != nil {
				changed = f.Value.String() != f.DefValue
			} else {
				fmt.Println("+", fullKey, "skipped")
				continue
			}

			// in any of these cases, don't store the key in the file
			if setup || hidden || (!user && !changed && !overrideExists) {
				fmt.Println("-", fullKey, setup, hidden, user, changed, overrideExists)
				delete(settings, key)
				continue
			}
			fmt.Println("+", fullKey, "valid")
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
