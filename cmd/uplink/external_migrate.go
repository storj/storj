// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/zeebo/errs"
	"github.com/zeebo/ini"
	"gopkg.in/yaml.v3"
)

// migrate attempts to create the config file from the old config file if the
// config file does not exist. It will only attempt to do so at most once
// and so calls to migrate are idempotent.
func (ex *external) migrate() (err error) {
	configFile, err := ex.ConfigFile()
	if err != nil {
		return errs.Wrap(err)
	}

	if ex.migration.migrated {
		return ex.migration.err
	}
	ex.migration.migrated = true

	// save any migration error that may have happened
	defer func() { ex.migration.err = err }()

	// if the config file exists, there is no need to migrate
	if _, err := os.Stat(configFile); err == nil {
		return nil
	}

	// if the old config file does not exist, we cannot migrate
	legacyFh, err := os.Open(ex.legacyConfigFile())
	if err != nil {
		return nil
	}
	defer func() { _ = legacyFh.Close() }()

	// load the information necessary to write the new config from
	// the old file.
	defaultName, accesses, entries, err := ex.parseLegacyConfig(legacyFh)
	if err != nil {
		return errs.Wrap(err)
	}

	// ensure the loaded config matches the new format where the default
	// name is actually a name that always points into the accesses map.
	defaultName, _ = checkAccessMapping(defaultName, accesses)

	// ensure the directory that will hold the config files exists.
	if err := os.MkdirAll(ex.dirs.current, 0755); err != nil {
		return errs.Wrap(err)
	}

	// first, create and write the access file. that way, if there's an error
	// creating the config file, we will recreate this file.
	if err := ex.SaveAccessInfo(defaultName, accesses); err != nil {
		return errs.Wrap(err)
	}

	// now, write out the config file from the stored entries.
	if err := ex.saveConfig(entries); err != nil {
		return errs.Wrap(err)
	}

	// migration complete!
	return nil
}

// parseLegacyConfig loads the default access name, the map of available accesses, and
// a list of config entries from the yaml file in the reader.
func (ex *external) parseLegacyConfig(r io.Reader) (string, map[string]string, []ini.Entry, error) {
	defaultName := ""
	accesses := make(map[string]string)
	entries := make([]ini.Entry, 0)

	// load the old config if possible and write out a new config
	var node yaml.Node
	if err := yaml.NewDecoder(r).Decode(&node); err != nil {
		return "", nil, nil, errs.Wrap(err)
	}

	// walking a yaml node is unfortunately recursive, so we have to do this
	// predeclaration trick to do a recursive inline function.
	var walk func(*yaml.Node, []string) error
	walk = func(node *yaml.Node, stack []string) error {
		if node.Kind != yaml.MappingNode {
			return errs.New("unexpected non-map node in yaml document")
		} else if len(node.Content)%2 != 0 {
			return errs.New("map has odd number of content entries in yaml document")
		}

		section := strings.Join(stack, ".")

		// walk the map entries in pairs. the first entry is the key, and the second is
		// the value.
		for i := 0; i < len(node.Content); i += 2 {
			keyn, valuen := node.Content[i], node.Content[i+1]
			key, value := keyn.Value, valuen.Value

			// we don't support key kinds other than scalar. yaml may not either. shrug.
			if keyn.Kind != yaml.ScalarNode {
				return errs.New("map has non-scalar key type")
			}

			switch valuen.Kind {
			case yaml.ScalarNode:
				// we want to intercept the access and accesses values from the config
				// because they go into a separate file now. check for keys that match
				// one of those and stuff them away outside of entries.
				if key == "access" {
					defaultName = value
				} else if strings.HasPrefix(key, "accesses.") {
					accesses[key[len("accesses."):]] = value
				} else if section == "accesses" {
					accesses[key] = value
				} else {
					entries = append(entries, ini.Entry{
						Key:     key,
						Value:   value,
						Section: section,
					})
				}

			case yaml.MappingNode:
				if err := walk(valuen, append(stack, key)); err != nil {
					return err
				}

			default:
				return errs.New("yaml map contains non-scalar or map content entry")
			}
		}

		return nil
	}

	if node.Kind != yaml.DocumentNode {
		return "", nil, nil, errs.New("yaml root node is not document")
	}
	if len(node.Content) != 1 || node.Content[0].Kind != yaml.MappingNode {
		return "", nil, nil, errs.New("yaml root node does not contain a single map")
	}
	if err := walk(node.Content[0], nil); err != nil {
		return "", nil, nil, err
	}

	return defaultName, accesses, entries, nil
}

func checkAccessMapping(accessName string, accesses map[string]string) (newName string, ok bool) {
	if _, ok := accesses[accessName]; ok {
		return accessName, false
	}

	// the only reason the name would not be present is because
	// it's actually an access grant. we could check that, but
	// if an error happens, the old config must be broken in
	// a way we can't repair, anyway, so let's just keep it the
	// same amount of broken. so, all we need to do is pick a
	// name that doesn't yet exist.

	newName = "main"
	for i := 2; ; i++ {
		if _, ok := accesses[newName]; !ok {
			break
		}
		newName = fmt.Sprintf("main-%d", i)
	}

	accesses[newName] = accessName
	return newName, true
}
