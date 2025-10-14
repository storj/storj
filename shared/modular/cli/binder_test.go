// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/clingy"

	"storj.io/common/memory"
)

// Test configuration structures
type SimpleConfig struct {
	StringField   string        `help:"string field"`
	IntField      int           `help:"int field" default:"42"`
	BoolField     bool          `help:"bool field" default:"true"`
	DurationField time.Duration `help:"duration field" default:"10s"`
	Float64Field  float64       `help:"float64 field" default:"3.14"`
	Uint64Field   uint64        `help:"uint64 field" default:"100"`
}

type ConfigWithTags struct {
	NoFlagField    string `noflag:"true" help:"field that should not create a flag"`
	NoPrefixField  string `noprefix:"true" help:"field without prefix"`
	DevDefault     string `help:"field with dev default" default:"release" devDefault:"dev"`
	ReleaseDefault string `help:"field with release default" releaseDefault:"production"`
}

type NestedConfig struct {
	DatabaseSettings DatabaseConfig   `help:"database configuration"`
	Server           TestServerConfig `help:"server configuration"`
}

type NestedPointerConfig struct {
	Database *DatabaseConfig `help:"database configuration"`
}

type DatabaseConfig struct {
	Host string `help:"database host" default:"localhost"`
	Port int    `help:"database port" default:"5432"`
}

type TestServerConfig struct {
	Name       string
	URL        string
	CaCertPath string
}

type AnonymousEmbedConfig struct {
	SimpleConfig
	ExtraField string `help:"extra field"`
}

type NodeSelectionConfig struct {
	RequiredFreeSpace memory.Size `help:"required free space" default:"10GB"`
}

func TestBindConfig_SimpleFields(t *testing.T) {
	params := newMockParameters()
	cfg := &ConfigSupport{identityDir: "/test/identity"}
	config := &SimpleConfig{}

	// Test that bindConfig doesn't panic and registers flags
	require.NotPanics(t, func() {
		bindConfig(params, "", reflect.ValueOf(config), cfg)
	})

	// Check that flags were registered
	expectedFlags := []string{
		"string-field",
		"int-field",
		"bool-field",
		"duration-field",
		"float64field", // Note: camelToSnakeCase doesn't handle number->letter transitions
		"uint64field",  // Note: camelToSnakeCase doesn't handle number->letter transitions
	}

	for _, flagName := range expectedFlags {
		_, exists := params.flags[flagName]
		require.True(t, exists, "flag %s should exist", flagName)
	}
}

func TestBindConfig_WithPrefix(t *testing.T) {
	params := newMockParameters()
	cfg := &ConfigSupport{identityDir: "/test/identity"}
	config := &SimpleConfig{}

	require.NotPanics(t, func() {
		bindConfig(params, "test", reflect.ValueOf(config), cfg)
	})

	// Check that flags have prefix
	expectedFlags := []string{
		"test.string-field",
		"test.int-field",
		"test.bool-field",
		"test.duration-field",
		"test.float64field", // Note: camelToSnakeCase doesn't handle number->letter transitions
		"test.uint64field",  // Note: camelToSnakeCase doesn't handle number->letter transitions
	}

	for _, flagName := range expectedFlags {
		_, exists := params.flags[flagName]
		require.True(t, exists, "flag %s should exist", flagName)
	}
}

func TestBindConfig_Conversions(t *testing.T) {
	t.Run("from string", func(t *testing.T) {
		params := newMockParameters()
		cfg := &ConfigSupport{identityDir: "/test/identity"}
		config := &SimpleConfig{}

		params.values = map[string]any{
			"test.string-field":   "foo",
			"test.int-field":      "9",
			"test.bool-field":     "true",
			"test.duration-field": "1s",
			"test.float64field":   "1.5",
			"test.uint64field":    "10",
		}

		require.NotPanics(t, func() {
			bindConfig(params, "test", reflect.ValueOf(config), cfg)
		})

		require.Equal(t, "foo", config.StringField)
		require.Equal(t, 9, config.IntField)
		require.Equal(t, true, config.BoolField)
		require.Equal(t, time.Second, config.DurationField)
		require.Equal(t, 1.5, config.Float64Field)
		require.Equal(t, uint64(10), config.Uint64Field)
	})
	t.Run("from raw", func(t *testing.T) {
		params := newMockParameters()
		cfg := &ConfigSupport{identityDir: "/test/identity"}
		config := &SimpleConfig{}

		params.values = map[string]any{
			"test.string-field": "foo",
			"test.int-field":    9,
			"test.bool-field":   true,
			"test.float64field": 1.5,
			"test.uint64field":  uint64(10),
		}

		require.NotPanics(t, func() {
			bindConfig(params, "test", reflect.ValueOf(config), cfg)
		})

		require.Equal(t, "foo", config.StringField)
		require.Equal(t, 9, config.IntField)
		require.Equal(t, true, config.BoolField)
		require.Equal(t, 1.5, config.Float64Field)
		require.Equal(t, uint64(10), config.Uint64Field)
	})
}

func TestBindConfig_Tags(t *testing.T) {
	params := newMockParameters()
	cfg := &ConfigSupport{identityDir: "/test/identity"}
	config := &ConfigWithTags{}

	require.NotPanics(t, func() {
		bindConfig(params, "prefix", reflect.ValueOf(config), cfg)
	})

	// noflag field should not create a flag
	_, exists := params.flags["prefix.no-flag-field"]
	require.False(t, exists, "noflag field should not create a flag")

	// noprefix field should not have prefix
	_, exists = params.flags["no-prefix-field"]
	require.True(t, exists, "noprefix field should exist without prefix")

	// releaseDefault should create a flag
	_, exists = params.flags["prefix.release-default"]
	require.True(t, exists, "releaseDefault field should create a flag")
}

func TestBindConfig_NestedStructs(t *testing.T) {
	params := newMockParameters()
	cfg := &ConfigSupport{identityDir: "/test/identity"}
	config := &NestedConfig{}

	require.NotPanics(t, func() {
		bindConfig(params, "", reflect.ValueOf(config), cfg)
	})

	// Check nested field flags
	expectedFlags := []string{
		"database-settings.host",
		"database-settings.port",
		"server.name",
		"server.url",
		"server.ca-cert-path",
	}
	for _, flagName := range expectedFlags {
		_, exists := params.flags[flagName]
		require.True(t, exists, "nested flag %s should exist", flagName)
	}
}

func TestBindConfig_NestedSPointerStructs(t *testing.T) {
	params := newMockParameters()
	cfg := &ConfigSupport{identityDir: "/test/identity"}
	config := &NestedPointerConfig{}

	require.NotPanics(t, func() {
		bindConfig(params, "", reflect.ValueOf(config), cfg)
	})

	// Check nested field flags
	expectedFlags := []string{
		"database.host",
		"database.port",
	}

	for _, flagName := range expectedFlags {
		_, exists := params.flags[flagName]
		require.True(t, exists, "nested flag %s should exist", flagName)
	}
}

func TestBindConfig_AnonymousEmbedded(t *testing.T) {
	params := newMockParameters()
	cfg := &ConfigSupport{identityDir: "/test/identity"}
	config := &AnonymousEmbedConfig{}

	// Test that anonymous embedded structs are handled correctly
	require.NotPanics(t, func() {
		bindConfig(params, "", reflect.ValueOf(config), cfg)
	})

	// Check that flags from both the embedded struct and the parent are created
	expectedFlags := []string{
		"string-field",   // from embedded SimpleConfig
		"int-field",      // from embedded SimpleConfig
		"bool-field",     // from embedded SimpleConfig
		"duration-field", // from embedded SimpleConfig
		"float64field",   // from embedded SimpleConfig
		"uint64field",    // from embedded SimpleConfig
		"extra-field",    // from AnonymousEmbedConfig
	}

	for _, flagName := range expectedFlags {
		_, exists := params.flags[flagName]
		require.True(t, exists, "flag %s should exist", flagName)
	}
}

func TestBindConfig_MemorySize(t *testing.T) {
	params := newMockParameters()
	cfg := &ConfigSupport{identityDir: "/test/identity"}
	config := &NodeSelectionConfig{}

	require.NotPanics(t, func() {
		bindConfig(params, "", reflect.ValueOf(config), cfg)
	})

	expectedFlags := []string{
		"required-free-space",
	}

	for _, flagName := range expectedFlags {
		_, exists := params.flags[flagName]
		require.True(t, exists, "flag %s should exist", flagName)
	}
}

func TestBindConfig_IdentityDirReplacement(t *testing.T) {
	params := newMockParameters()
	cfg := &ConfigSupport{identityDir: "/custom/identity"}

	type ConfigWithIdentityDir struct {
		CertPath string `help:"cert path" default:"$IDENTITYDIR/cert.pem"`
	}

	config := &ConfigWithIdentityDir{}

	require.NotPanics(t, func() {
		bindConfig(params, "", reflect.ValueOf(config), cfg)
	})

	// Check that the flag was created (the identity dir replacement happens in the actual Flag call)
	_, exists := params.flags["cert-path"]
	require.True(t, exists)
}

func TestBindConfig_PanicCases(t *testing.T) {
	params := newMockParameters()
	cfg := &ConfigSupport{identityDir: "/test/identity"}

	// Test panic when not passing a pointer to struct
	t.Run("non-struct panic", func(t *testing.T) {
		require.Panics(t, func() {
			notAStruct := "string"
			bindConfig(params, "", reflect.ValueOf(&notAStruct), cfg)
		})
	})

	// Test panic with unsupported field type
	t.Run("unsupported type panic", func(t *testing.T) {
		type UnsupportedConfig struct {
			UnsupportedField chan int `help:"unsupported field"`
		}
		config := &UnsupportedConfig{}
		require.Panics(t, func() {
			bindConfig(params, "", reflect.ValueOf(config), cfg)
		})
	})

	// Unexported fields are ignored without panic
	t.Run("unexported type without panic", func(t *testing.T) {
		type UnexportedConfig struct {
			unexported int `help:"unsupported field"`
		}
		config := &UnexportedConfig{
			unexported: 12,
		}

		// just for the linter, who is worried about unused + unexported field
		require.Equal(t, 12, config.unexported)

		bindConfig(params, "", reflect.ValueOf(config), cfg)
	})
}

func TestSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SimpleField", "simple_field"},
		{"HTTPServer", "http_server"},
		{"XMLParser", "xml_parser"},
		{"IOReader", "io_reader"},
		{"A", "a"},
		{"", ""},
		{"HTTPSProxy", "https_proxy"},
		{"APIKey", "api_key"},
		{"URLPath", "url_path"},
		{"unexported", "unexported"},
		{"nodeID", "node_id"},
	}

	for _, test := range tests {
		result := camelToSnakeCase(test.input)
		require.Equal(t, test.expected, result, "camelToSnakeCase(%s) should return %s", test.input, test.expected)
	}
}

func TestHyphenate(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple_field", "simple-field"},
		{"http_server", "http-server"},
		{"", ""},
		{"no_underscore", "no-underscore"},
		{"multiple_under_scores", "multiple-under-scores"},
		{"unexported", "unexported"},
		{"nodeID", "node-id"},
	}

	for _, test := range tests {
		result := snakeToHyphenatedCase(test.input)
		require.Equal(t, test.expected, result, "snakeToHyphenatedCase(%s) should return %s", test.input, test.expected)
	}
}

// mockParameters implements clingy.Parameters for testing
type mockParameters struct {
	flags  map[string]interface{}
	values map[string]interface{}
}

func newMockParameters() *mockParameters {
	return &mockParameters{
		flags: make(map[string]interface{}),
	}
}

func (m *mockParameters) Flag(name, help string, def interface{}, opts ...clingy.Option) interface{} {
	// Store that this flag was registered
	m.flags[name] = def

	// Special case: for unsupported-field test, return a value to trigger type checking
	if name == "unsupported-field" {
		return "dummy" // This will cause the type switch to be executed
	}

	// Return nil to skip value assignment in bindConfig (this causes early continue)
	return m.values[name]
}

func (m *mockParameters) Arg(name, help string, opts ...clingy.Option) interface{} {
	return ""
}

func (m *mockParameters) Args(name, help string, n int, opts ...clingy.Option) []interface{} {
	return nil
}

func (m *mockParameters) Break() {
	// No-op for testing
}
