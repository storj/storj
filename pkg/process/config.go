// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"sort"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// SaveConfig will save only the user-specific flags with default values to
// outfile with specific values specified in 'overrides' overridden.
func SaveConfig(cmd *cobra.Command, outfile string, overrides map[string]interface{}) error {
	return saveConfig(cmd, outfile, overrides, false)
}

// SaveConfigWithAllDefaults will save all flags with default values to outfile
// with specific values specified in 'overrides' overridden.
func SaveConfigWithAllDefaults(cmd *cobra.Command, outfile string, overrides map[string]interface{}) error {
	return saveConfig(cmd, outfile, overrides, true)
}

func saveConfig(cmd *cobra.Command, outfile string, overrides map[string]interface{}, saveAllDefaults bool) error {
	// we previously used Viper here, but switched to a custom serializer to allow comments
	// todo:  switch back to Viper once go-yaml v3 is released and its supports writing comments?
	flagset := cmd.Flags()
	flagset.AddFlagSet(pflag.CommandLine)

	// sort keys
	var keys []string
	flagset.VisitAll(func(f *pflag.Flag) { keys = append(keys, f.Name) })
	sort.Strings(keys)

	// serialize
	w := new(bytes.Buffer)
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

		value := f.Value.String()
		if overriddenValue != nil {
			value = fmt.Sprintf("%v", overriddenValue)
		}
		// print usage info
		if f.Usage != "" {
			fmt.Fprintf(w, "# %s\n", f.Usage)
		}
		// print commented key (beginning of value assignement line)
		if readBoolAnnotation(f, "user") || f.Changed || overrideExist {
			fmt.Fprintf(w, "%s: ", k)
		} else {
			fmt.Fprintf(w, "# %s: ", k)
		}
		// print value (remainder of value assignement line)
		switch f.Value.Type() {
		case "string":
			// save ourselves 250+ lines of code and just double quote strings
			fmt.Fprintf(w, "%q\n\n", value)
		default:
			// assume that everything else doesn't have fancy control characters
			fmt.Fprintf(w, "%s\n\n", value)
		}
	}

	err := ioutil.WriteFile(outfile, w.Bytes(), os.FileMode(0644))
	if err != nil {
		return err
	}
	fmt.Println("Your configuration is saved to:", outfile)
	return nil
}

func readBoolAnnotation(flag *pflag.Flag, key string) bool {
	annotation := flag.Annotations[key]
	return len(annotation) > 0 && annotation[0] == "true"
}
