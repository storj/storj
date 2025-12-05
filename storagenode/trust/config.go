// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package trust

import (
	"errors"
	"strings"
	"time"
)

// Config is the trust configuration.
type Config struct {
	Sources         Sources       `help:"list of trust sources" devDefault:"" releaseDefault:"https://static.storj.io/dcs-satellites"`
	Exclusions      Exclusions    `help:"list of trust exclusions" devDefault:"" releaseDefault:""`
	RefreshInterval time.Duration `help:"how often the trust pool should be refreshed" default:"6h"`
	CachePath       string        `help:"file path where trust lists should be cached" default:"${CONFDIR}/trust-cache.json"`
}

// Sources is a list of sources that implements pflag.Value.
type Sources []Source

// String returns the string representation of the config.
func (sources Sources) String() string {
	s := make([]string, 0, len(sources))
	for _, source := range sources {
		s = append(s, source.String())
	}
	return strings.Join(s, ",")
}

// Set implements pflag.Value by parsing a comma separated list of sources.
func (sources *Sources) Set(value string) error {
	var entries []string
	if value != "" {
		entries = strings.Split(value, ",")
	}

	var toSet []Source
	for _, entry := range entries {
		source, err := NewSource(entry)
		if err != nil {
			return Error.New("invalid source %q: %w", entry, errors.Unwrap(err))
		}
		toSet = append(toSet, source)
	}

	*sources = toSet
	return nil
}

// Type returns the type of the pflag.Value.
func (sources Sources) Type() string {
	return "trust-sources"
}

// Exclusions is a list of excluding rules that implements pflag.Value.
type Exclusions struct {
	Rules Rules
}

// String returns the string representation of the config.
func (exclusions *Exclusions) String() string {
	s := make([]string, 0, len(exclusions.Rules))
	for _, rule := range exclusions.Rules {
		s = append(s, rule.String())
	}
	return strings.Join(s, ",")
}

// Set implements pflag.Value by parsing a comma separated list of exclusions.
func (exclusions *Exclusions) Set(value string) error {
	var entries []string
	if value != "" {
		entries = strings.Split(value, ",")
	}

	var rules Rules
	for _, entry := range entries {
		rule, err := NewExcluder(entry)
		if err != nil {
			return Error.New("invalid exclusion %q: %w", entry, errors.Unwrap(err))
		}
		rules = append(rules, rule)
	}

	exclusions.Rules = rules
	return nil
}

// Type returns the type of the pflag.Value.
func (exclusions Exclusions) Type() string {
	return "trust-exclusions"
}
