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
		cfgPath := filepath.Join(c.configDir, "config.yaml")
		_, err := os.Stat(cfgPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return errs.Wrap(err)
		}
		cfgContent, err := os.ReadFile(cfgPath)
		if err != nil {
			return errs.Wrap(err)
		}

		err = yaml.Unmarshal(cfgContent, &c.raw)
		if err != nil {
			return errs.Wrap(err)
		}
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
		cfgPath := filepath.Join(c.configDir, "config.yaml")

		if _, err := os.Stat(cfgPath); err == nil {
			vip.SetConfigFile(cfgPath)
			if err := vip.ReadInConfig(); err != nil {
				return []string{}, err
			}
		}
		c.settings = vip.AllSettings()
	}

	val := getRecursive(c.settings, strings.Split(name, "."))
	if val != nil {
		return []string{fmt.Sprintf("%v", val)}, nil
	}
	return []string{}, nil
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
