// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"
	"gopkg.in/yaml.v2"
)

// ConfigDir is a view of the ConfigSupport.configDir.
type ConfigDir struct {
	Dir string
}

// ConfigSupport is a clingy helper, which loads the configuration.
type ConfigSupport struct {
	configDir   string
	identityDir string

	// raw YAML configuration values loaded from config.yaml
	raw map[interface{}]interface{}

	// settings is a map of all configuration values, parsed by viper (including env variables)
	settings map[string]any
}

// Setup register the config-dir flag with clingy and reads configuration
func (c *ConfigSupport) Setup(cmds clingy.Commands) {
	c.configDir = cmds.Flag("config-dir", "main directory for configuration", "").(string)
	c.identityDir = cmds.Flag("identity-dir", "main directory for searching for identity files", "").(string)
}

// GetSubtree returns the configuration tree (in raw parser YAML format) for a given prefix.
func (c *ConfigSupport) GetSubtree(prefix string, target interface{}) error {
	if c.raw == nil {
		c.raw = map[interface{}]interface{}{}

		// config.yaml takes precedence over secrets.yaml: load config.yaml first,
		// then fill in only the keys that are missing from secrets.yaml.
		config, err := readRawConfig(filepath.Join(c.configDir, "config.yaml"))
		if err != nil {
			return err
		}
		mergeRaw(c.raw, config)

		secrets, err := readRawConfig(filepath.Join(c.configDir, "secrets.yaml"))
		if err != nil {
			return err
		}
		mergeRaw(c.raw, secrets)
	}
	subtree := selectTreeRecursive(prefix, c.raw)
	if subtree == nil {
		return nil
	}
	out, err := yaml.Marshal(subtree)
	if err != nil {
		return errs.Wrap(err)
	}

	err = yaml.Unmarshal(out, target)
	if err != nil {
		return errs.Wrap(err)
	}

	return nil
}

// selectTreeRecursive is a helper function that recursively finds a subtree from the raw configuration map.
func selectTreeRecursive(prefix string, raw map[interface{}]interface{}) interface{} {
	if val, found := raw[prefix]; found {
		return val
	}
	level, remaining, _ := strings.Cut(prefix, ".")
	if val, found := raw[level]; found {
		return selectTreeRecursive(remaining, val.(map[interface{}]interface{}))
	}
	return nil
}

// readRawConfig reads a YAML configuration file into a raw parser map. It returns
// a nil map (and no error) when the file doesn't exist.
func readRawConfig(path string) (map[any]any, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errs.Wrap(err)
	}
	raw := map[any]any{}
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, errs.Wrap(err)
	}
	return raw, nil
}

// mergeRaw deep-merges src into dst. Keys already present in dst take precedence;
// nested maps are merged recursively.
func mergeRaw(dst, src map[any]any) {
	for k, v := range src {
		if existing, ok := dst[k]; ok {
			existingMap, ok1 := existing.(map[any]any)
			srcMap, ok2 := v.(map[any]any)
			if ok1 && ok2 {
				mergeRaw(existingMap, srcMap)
			}
			// otherwise dst wins (precedence), so leave it untouched.
			continue
		}
		dst[k] = v
	}
}

// GetValue is a clingy helper, which returns the value of a given configuration key, using viper.
func (c *ConfigSupport) GetValue(name string) (vals []string, err error) {
	if c.settings == nil {
		vip := viper.New()

		// We couldn't use vip.AutomaticEnv() here, as the configuration key may not be available here.
		// clingy handles the parameters in an independent way, pflag doesn't include the flags, therefore
		// viper doesn't know about the key
		// viper doesn't scan all the STORJ_ environment variables, only checks the values for known keys
		// instead of using vip.AutomaticEnv(), we check the environment key in the Setup phase of clingy
		// see getFlagValue in binder.go
		// Load secrets.yaml first, then merge config.yaml on top so that config.yaml
		// takes precedence when a key exists in both files.
		for _, name := range []string{"secrets.yaml", "config.yaml"} {
			cfgPath := filepath.Join(c.configDir, name)
			if _, err := os.Stat(cfgPath); err != nil {
				continue
			}
			vip.SetConfigFile(cfgPath)
			if err := vip.MergeInConfig(); err != nil {
				panic(fmt.Sprintf("failed to read config file %s: %v", cfgPath, err))
			}
		}
		c.settings = vip.AllSettings()
	}

	val := getRecursive(c.settings, strings.Split(name, "."))
	if val == nil {
		return []string{}, nil
	}
	// YAML list values arrive as a slice. The binder expects []string fields as a
	// single comma-separated string (see the []string case in binder.go), so join
	// the elements with commas instead of stringifying the slice with fmt's "[a b]"
	// representation, which would otherwise corrupt the values.
	switch list := val.(type) {
	case []interface{}:
		parts := make([]string, len(list))
		for i, item := range list {
			parts[i] = fmt.Sprintf("%v", item)
		}
		return []string{strings.Join(parts, ",")}, nil
	case []string:
		return []string{strings.Join(list, ",")}, nil
	default:
		return []string{fmt.Sprintf("%v", val)}, nil
	}
}

func getRecursive(settings map[string]any, split []string) interface{} {
	if len(split) == 0 {
		return nil
	}
	if val, ok := settings[strings.Join(split, ".")]; ok {
		return val
	}
	if val, ok := settings[split[0]]; ok {
		if subtree, ok := val.(map[string]interface{}); ok {
			return getRecursive(subtree, split[1:])
		}
		return val
	}
	return nil
}
