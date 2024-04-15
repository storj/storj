// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package jsoniter

// Config is a stub to avoid importing json-iterator/go.
type Config struct {
	EscapeHTML  bool
	SortMapKeys bool
	UseNumber   bool
}

// ConfigCompatibleWithStandardLibrary represents a panicking implementation.
var ConfigCompatibleWithStandardLibrary Config

// Unmarshal panicks.
func (Config) Unmarshal(data []byte, v any) error { panic("should not be called") }

// Marshal panicks.
func (Config) Marshal(v any) ([]byte, error) { panic("should not be called") }

// Froze panicks.
func (Config) Froze() Config { panic("should not be called") }
