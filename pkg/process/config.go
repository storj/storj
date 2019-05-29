// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// SaveConfig will save only the user-specific flags with default values to
// config files in the directory with specific values specified in 'overrides' overridden.
func SaveConfig(cmd *cobra.Command, directory string, overrides map[string]interface{}) error {
	return saveConfig(cmd, directory, overrides, false)
}

// SaveConfigWithAllDefaults will save all flags with default values to config
// files in the directory with specific values specified in 'overrides' overridden.
func SaveConfigWithAllDefaults(cmd *cobra.Command, directory string, overrides map[string]interface{}) error {
	return saveConfig(cmd, directory, overrides, true)
}

func saveConfig(cmd *cobra.Command, directory string, overrides map[string]interface{}, saveAllDefaults bool) error {
	// we previously used Viper here, but switched to a custom serializer to allow comments
	// todo:  switch back to Viper once go-yaml v3 is released and its supports writing comments?
	flagset := cmd.Flags()
	flagset.AddFlagSet(pflag.CommandLine)

	// sort keys
	var keys []string
	flagset.VisitAll(func(f *pflag.Flag) { keys = append(keys, f.Name) })
	sort.Strings(keys)

	// serialize
	configs, secureConfigs := new(bytes.Buffer), new(bytes.Buffer)
	for _, k := range keys {
		f := flagset.Lookup(k)
		if readBoolAnnotation(f, "setup") {
			continue
		}

		var overriddenValue interface{}
		var overrideExist bool
		if overrides != nil {
			overriddenValue, overrideExist = overrides[k]
		}

		if !saveAllDefaults && !readBoolAnnotation(f, "user") && !f.Changed && !overrideExist {
			continue
		}

		writer := configs
		if readBoolAnnotation(f, "secure") {
			writer = secureConfigs
		}

		value := f.Value.String()
		if overriddenValue != nil {
			value = fmt.Sprintf("%v", overriddenValue)
		}
		// print usage info
		if f.Usage != "" {
			fmt.Fprintf(writer, "# %s\n", f.Usage)
		}
		// print commented key (beginning of value assignement line)
		if readBoolAnnotation(f, "user") || f.Changed || overrideExist {
			fmt.Fprintf(writer, "%s: ", k)
		} else {
			fmt.Fprintf(writer, "# %s: ", k)
		}
		// print value (remainder of value assignement line)
		switch f.Value.Type() {
		case "string":
			// save ourselves 250+ lines of code and just double quote strings
			fmt.Fprintf(writer, "%q\n\n", value)
		default:
			// assume that everything else doesn't have fancy control characters
			fmt.Fprintf(writer, "%s\n\n", value)
		}
	}

	configsFile := filepath.Join(directory, DefaultCfgFilename)
	err := ioutil.WriteFile(configsFile, configs.Bytes(), os.FileMode(0644))
	if err != nil {
		return err
	}
	fmt.Println("Your configuration is saved to:", configsFile)

	if len(secureConfigs.Bytes()) > 0 {
		secureConfigsFile := filepath.Join(directory, DefaultSecureCfgFilename)
		err := ioutil.WriteFile(secureConfigsFile, secureConfigs.Bytes(), os.FileMode(0600))
		if err != nil {
			return err
		}
		fmt.Println("Your secure configuration is saved to:", secureConfigsFile)
	}

	return nil
}

func readBoolAnnotation(flag *pflag.Flag, key string) bool {
	annotation := flag.Annotations[key]
	return len(annotation) > 0 && annotation[0] == "true"
}
