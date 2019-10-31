// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"strings"
	"time"

	"github.com/zeebo/errs"
)

// Config is the trust configuration
type Config struct {
	List            ListConfig    `help:"list of trust sources and filters" devDefault:"" releaseDefault:"https://tardigrade.io/trusted-satellites"`
	RefreshInterval time.Duration `help:"how often the trust pool should be refreshed" default:"6h"`
	CachePath       string        `help:"file path where trust lists should be cached" default:"${CONFDIR}/trust-cache.json"`
}

// ListConfig is the trust list configuration. It implements a pflag.Value.
type ListConfig struct {
	// raw config value
	value string

	Sources []Source
	Filter  *Filter
}

// String returns the string representation of the config
func (config *ListConfig) String() string {
	return config.value
}

// Set sets the configuration value by parsing the comma separated list into
// sources and filters.
func (config *ListConfig) Set(value string) error {
	var list []string
	if value != "" {
		list = strings.Split(value, ",")
	}
	sources, filter, err := ParseConfigList(list)
	if err != nil {
		return err
	}
	config.value = value
	config.Sources = sources
	config.Filter = filter
	return nil
}

// Type returns the type of the pflag.Value
func (config *ListConfig) Type() string {
	return "trust-list"
}

// ParseConfigList parses a list of strings according to the Satellite
// Selection specification and returns a set of trust sources and a configured
// filter.
func ParseConfigList(list []string) ([]Source, *Filter, error) {
	var sources []Source
	filter := NewFilter()

	for i, entry := range list {
		if strings.HasPrefix(entry, "!") {
			if err := filter.Add(entry[1:]); err != nil {
				return nil, nil, Error.New("invalid filter at position %d: %v", i, errs.Unwrap(err))
			}
			continue
		}

		source, err := NewSource(entry)
		if err != nil {
			return nil, nil, Error.New("invalid source at position %d: %v", i, errs.Unwrap(err))
		}
		sources = append(sources, source)
	}
	return sources, filter, nil
}
