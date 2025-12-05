// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package mudplanet

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Custom type implementing pflag.Value interface.
type CustomFlag string

func (c *CustomFlag) String() string {
	return string(*c)
}

func (c *CustomFlag) Set(value string) error {
	*c = CustomFlag(value + "_set")
	return nil
}

func (c *CustomFlag) Type() string {
	return "customflag"
}

// Test struct with various field types and default values.
type TestConfig struct {
	StringField      string        `default:"test string"`
	IntField         int           `default:"42"`
	Int64Field       int64         `default:"10000000"`
	BoolField        bool          `default:"true"`
	DurationField    time.Duration `default:"5m"`
	FloatField       float64       `default:"3.14"`
	StringWithVars   string        `default:"$CONFDIR/file.txt"`
	CustomField      CustomFlag    `default:"custom"`
	EmptyField       string        // No default value
	DevField         string        `devDefault:"development"`
	TestField        string        `testDefault:"testing"`
	ReleaseField     string        `releaseDefault:"production"`
	MultipleDefaults string        `testDefault:"test" devDefault:"dev" default:"fallback"`
	NestedStruct     NestedConfig
	StringSlice      []string `default:"foo,bar"`
}

// Nested configuration struct.
type NestedConfig struct {
	NestedString string `default:"nested value"`
	NestedInt    int    `default:"123"`
}

func TestInjectDefault(t *testing.T) {
	// Create a new TestConfig
	cfg := &TestConfig{}
	val := reflect.ValueOf(cfg).Elem()
	workDir := "/test/workdir"

	// Call injectDefault
	injectDefault(t, val, workDir)

	// Verify all fields have correct values
	assert.Equal(t, "test string", cfg.StringField)
	assert.Equal(t, 42, cfg.IntField)
	assert.Equal(t, int64(10000000), cfg.Int64Field)
	assert.Equal(t, true, cfg.BoolField)
	assert.Equal(t, 5*time.Minute, cfg.DurationField)
	assert.Equal(t, 3.14, cfg.FloatField)
	assert.Equal(t, "/test/workdir/file.txt", cfg.StringWithVars)
	assert.Equal(t, CustomFlag("custom_set"), cfg.CustomField)
	assert.Equal(t, "", cfg.EmptyField)
	assert.Equal(t, "development", cfg.DevField)
	assert.Equal(t, "testing", cfg.TestField)
	assert.Equal(t, "production", cfg.ReleaseField)
	assert.Equal(t, "test", cfg.MultipleDefaults) // testDefault has highest priority
	assert.Equal(t, []string{"foo", "bar"}, cfg.StringSlice)

	// Verify nested struct fields
	assert.Equal(t, "nested value", cfg.NestedStruct.NestedString)
	assert.Equal(t, 123, cfg.NestedStruct.NestedInt)
}

func TestInjectDefaultPriorityOrder(t *testing.T) {
	// Test to verify the priority order of default tags
	type PriorityConfig struct {
		Field1 string `testDefault:"test" devDefault:"dev" default:"default" releaseDefault:"release"`
		Field2 string `devDefault:"dev" default:"default" releaseDefault:"release"`
		Field3 string `default:"default" releaseDefault:"release"`
		Field4 string `releaseDefault:"release"`
		Field5 string `devDefault:"127.0.0.1" testDefault:""`
		Field6 string `devDefault:"127.0.0.1"`
	}

	cfg := &PriorityConfig{}
	val := reflect.ValueOf(cfg).Elem()
	injectDefault(t, val, "")

	assert.Equal(t, "test", cfg.Field1)    // testDefault has highest priority
	assert.Equal(t, "dev", cfg.Field2)     // devDefault has second priority
	assert.Equal(t, "default", cfg.Field3) // default has third priority
	assert.Equal(t, "release", cfg.Field4) // releaseDefault has lowest priority
	assert.Equal(t, "", cfg.Field5)
	assert.Equal(t, "127.0.0.1", cfg.Field6)
}

func TestInjectDefaultHostVariable(t *testing.T) {
	type HostConfig struct {
		Host string `default:"$HOST:8080"`
	}

	cfg := &HostConfig{}
	val := reflect.ValueOf(cfg).Elem()
	injectDefault(t, val, "")

	assert.Equal(t, "127.0.0.1:8080", cfg.Host)
}

func TestInjectDefaultConfDirVariables(t *testing.T) {
	type PathConfig struct {
		Path1 string `default:"$CONFDIR/path/to/file"`
		Path2 string `default:"${CONFDIR}/other/file"`
	}

	cfg := &PathConfig{}
	val := reflect.ValueOf(cfg).Elem()
	workDir := "/custom/dir"
	injectDefault(t, val, workDir)

	assert.Equal(t, "/custom/dir/path/to/file", cfg.Path1)
	assert.Equal(t, "/custom/dir/other/file", cfg.Path2)
}
