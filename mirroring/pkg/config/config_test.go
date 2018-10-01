package config

import (
	"testing"

	"github.com/magiconair/properties/assert"
	"go.etcd.io/etcd/pkg/testutil"
)

func TestNewConfig(t *testing.T) {
	config := NewConfig()
	assert.Equal(t, config, &Config{})
}

func TestWithDefaultOptions(t *testing.T) {
	testOptions := NewDefaultOptions("TestSource", true)
	config := NewConfig()
	config.WithDefaultOptions(testOptions)

	assert.Equal(t, config.DefaultOptions, testOptions)
	assert.Equal(t, config.DefaultOptions.DefaultSource, testOptions.DefaultSource)
	testutil.AssertTrue(t, config.DefaultOptions.ThrowImmediately)
}

func TestWithServer1Credentials(t *testing.T) {
	credentials := NewCredentials("testEndpoint", "testKey", "testKey")
	config := NewConfig()
	config.WithServer1Credentials(credentials)

	assert.Equal(t, config.Server1, credentials)

	assert.Equal(t, config.Server1.Endpoint, credentials.Endpoint)
	assert.Equal(t, config.Server1.AccessKey, credentials.AccessKey)
	assert.Equal(t, config.Server1.SecretKey, credentials.SecretKey)

}

func TestWithServer2Credentials(t *testing.T) {
	credentials := NewCredentials("testEndpoint", "testKey", "testKey")
	config := NewConfig()
	config.WithServer2Credentials(credentials)

	assert.Equal(t, config.Server2, credentials)

	assert.Equal(t, config.Server2.Endpoint, credentials.Endpoint)
	assert.Equal(t, config.Server2.AccessKey, credentials.AccessKey)
	assert.Equal(t, config.Server2.SecretKey, credentials.SecretKey)
}

func TestWithListOptions(t *testing.T) {
	config := NewConfig()
	options := NewDefaultOptions("testSource", true)

	config.WithListOptions(options, true)

	assert.Equal(t, config.ListOptions.DefaultOptions, options)
	assert.Equal(t, config.ListOptions.Merge, true)
}

func TestWithPutOptions(t *testing.T) {
	config := NewConfig()
	options := NewDefaultOptions("testSource", true)

	config.WithPutOptions(options, true)

	assert.Equal(t, config.PutOptions.DefaultOptions, options)
	assert.Equal(t, config.PutOptions.CreateBucketIfNotExist, true)
}

func TestWithGetObjectOptions(t *testing.T) {
	config := NewConfig()
	options := NewDefaultOptions("testSource", true)

	config.WithGetObjectOptions(options)

	assert.Equal(t, config.GetObjectOptions.DefaultOptions, options)
}

func TestWithDeleteOptions(t *testing.T) {
	config := NewConfig()
	options := NewDefaultOptions("testSource", true)

	config.WithDeleteOptions(options)

	assert.Equal(t, config.DeleteOptions.DefaultOptions, options)
}

func TestWithCopyOptions(t *testing.T) {
	config := NewConfig()
	options := NewDefaultOptions("testSource", true)

	config.WithCopyOptions(options)

	assert.Equal(t, config.CopyOptions.DefaultOptions, options)
}

func TestGetDefaultSource(t *testing.T) {
	tests := []struct {
		name        string
		rootOptions *DefaultOptions
		argOptions  *DefaultOptions
		expected    string
	}{
		{
			name:        "Nil argOptions",
			rootOptions: NewDefaultOptions("server1", false),
			argOptions:  nil,
			expected:    "server1",
		},
		{
			name:        "Empty Arg options source",
			rootOptions: NewDefaultOptions("server1", false),
			argOptions:  NewDefaultOptions("", false),
			expected:    "server1",
		},
		{
			name:        "Valid case",
			rootOptions: NewDefaultOptions("server1", false),
			argOptions:  NewDefaultOptions("server2", false),
			expected:    "server2",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resultSource := test.rootOptions.GetDefaultSource(test.argOptions)
			assert.Equal(t, resultSource, test.expected)
		})
	}
}

func TestIsThrowImmediateError(t *testing.T) {
	tests := []struct {
		name        string
		rootOptions *DefaultOptions
		argOptions  *DefaultOptions
		expected    bool
	}{
		{
			name:        "Nil argOptions",
			rootOptions: NewDefaultOptions("", true),
			argOptions:  nil,
			expected:    true,
		},
		{
			name:        "Valid case",
			rootOptions: NewDefaultOptions("", false),
			argOptions:  NewDefaultOptions("", true),
			expected:    true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resultSource := test.rootOptions.IsThrowImmediateError(test.argOptions)
			assert.Equal(t, resultSource, test.expected)
		})
	}

}

func TestGetPrimeCredentials(t *testing.T) {
	tests := []struct {
		name        string
		server1Cred *Credentials
		server2Cred *Credentials
		server      string
		expected    *Credentials
	}{
		{
			name:   "Valid server1",
			server: "server1",
			server1Cred: &Credentials{
				Endpoint:  "enpoint",
				AccessKey: "accessKey",
				SecretKey: "secretKey",
			},
			expected: &Credentials{
				Endpoint:  "enpoint",
				AccessKey: "accessKey",
				SecretKey: "secretKey",
			},
		},
		{
			name:   "Valid server2",
			server: "server2",
			server2Cred: &Credentials{
				Endpoint:  "enpoint",
				AccessKey: "accessKey",
				SecretKey: "secretKey",
			},
			expected: &Credentials{
				Endpoint:  "enpoint",
				AccessKey: "accessKey",
				SecretKey: "secretKey",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := NewConfig()
			config.WithServer1Credentials(test.server1Cred)
			config.WithServer2Credentials(test.server2Cred)

			credentials := config.getPrimeCredentials(test.server)

			assert.Equal(t, credentials.AccessKey, test.expected.AccessKey)
			assert.Equal(t, credentials.SecretKey, test.expected.SecretKey)
			assert.Equal(t, credentials.Endpoint, test.expected.Endpoint)
		})
	}
}

func TestNilGetPrimeCredentials(t *testing.T) {
	config := NewConfig()
	credentials := config.getPrimeCredentials("")

	testutil.AssertNil(t, credentials)

}
func TestGetAlterCredentials(t *testing.T) {
	tests := []struct {
		name        string
		server1Cred *Credentials
		server2Cred *Credentials
		server      string
		expected    *Credentials
	}{
		{
			name:   "Valid server2",
			server: "server1",
			server2Cred: &Credentials{
				Endpoint:  "enpoint",
				AccessKey: "accessKey",
				SecretKey: "secretKey",
			},
			expected: &Credentials{
				Endpoint:  "enpoint",
				AccessKey: "accessKey",
				SecretKey: "secretKey",
			},
		},
		{
			name:   "Valid server1",
			server: "server2",
			server1Cred: &Credentials{
				Endpoint:  "enpoint",
				AccessKey: "accessKey",
				SecretKey: "secretKey",
			},
			expected: &Credentials{
				Endpoint:  "enpoint",
				AccessKey: "accessKey",
				SecretKey: "secretKey",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := NewConfig()
			config.WithServer1Credentials(test.server1Cred)
			config.WithServer2Credentials(test.server2Cred)

			credentials := config.getAlterCredentials(test.server)

			assert.Equal(t, credentials.AccessKey, test.expected.AccessKey)
			assert.Equal(t, credentials.SecretKey, test.expected.SecretKey)
			assert.Equal(t, credentials.Endpoint, test.expected.Endpoint)
		})
	}
}

func TestNilGetAlterCredentials(t *testing.T) {
	config := NewConfig()
	credentials := config.getAlterCredentials("")

	testutil.AssertNil(t, credentials)
}

func TestGetMergedPrimeCredentials(t *testing.T) {
	tests := []struct {
		name        string
		server1Cred *Credentials
		server2Cred *Credentials
		rootOptions *DefaultOptions
		argOptions  *DefaultOptions
		expected    *Credentials
	}{
		{
			name:        "Nil options",
			server1Cred: NewCredentials("server1End", "server1Acc", "server1Sec"),
			rootOptions: NewDefaultOptions("server1", true),
			argOptions:  nil,
			expected:    NewCredentials("server1End", "server1Acc", "server1Sec"),
		},
		{
			name:        "Empty default source",
			server1Cred: NewCredentials("server1End", "server1Acc", "server1Sec"),
			rootOptions: NewDefaultOptions("server1", true),
			argOptions:  NewDefaultOptions("", false),
			expected:    NewCredentials("server1End", "server1Acc", "server1Sec"),
		},
		{
			name:        "Valid case",
			server1Cred: NewCredentials("server1End", "server1Acc", "server1Sec"),
			server2Cred: NewCredentials("server2End", "server2Acc", "server2Sec"),
			rootOptions: NewDefaultOptions("server1", true),
			argOptions:  NewDefaultOptions("server2", false),
			expected:    NewCredentials("server2End", "server2Acc", "server2Sec"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := NewConfig().
				WithDefaultOptions(test.rootOptions).
				WithServer1Credentials(test.server1Cred).
				WithServer2Credentials(test.server2Cred)

			credentials := config.GetMergedPrimeCredentials(test.argOptions)

			assert.Equal(t, credentials.Endpoint, test.expected.Endpoint)
			assert.Equal(t, credentials.AccessKey, test.expected.AccessKey)
			assert.Equal(t, credentials.SecretKey, test.expected.SecretKey)
		})
	}
}


func TestGetMergedAlterCredentials(t *testing.T) {
	tests := []struct {
		name        string
		server1Cred *Credentials
		server2Cred *Credentials
		rootOptions *DefaultOptions
		argOptions  *DefaultOptions
		expected    *Credentials
	}{
		{
			name:        "Nil options",
			server2Cred: NewCredentials("server2End", "server2Acc", "server2Sec"),
			rootOptions: NewDefaultOptions("server1", true),
			argOptions:  nil,
			expected:    NewCredentials("server2End", "server2Acc", "server2Sec"),
		},
		{
			name:        "Empty default source",
			server2Cred: NewCredentials("server2End", "server2Acc", "server2Sec"),
			rootOptions: NewDefaultOptions("server1", true),
			argOptions:  NewDefaultOptions("", false),
			expected:    NewCredentials("server2End", "server2Acc", "server2Sec"),
		},
		{
			name:        "Valid case",
			server1Cred: NewCredentials("server1End", "server1Acc", "server1Sec"),
			server2Cred: NewCredentials("server2End", "server2Acc", "server2Sec"),
			rootOptions: NewDefaultOptions("server1", true),
			argOptions:  NewDefaultOptions("server2", false),
			expected:    NewCredentials("server1End", "server1Acc", "server1Sec"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := NewConfig().
				WithDefaultOptions(test.rootOptions).
				WithServer1Credentials(test.server1Cred).
				WithServer2Credentials(test.server2Cred)

			credentials := config.GetMergedAlterCredentials(test.argOptions)

			assert.Equal(t, credentials.Endpoint, test.expected.Endpoint)
			assert.Equal(t, credentials.AccessKey, test.expected.AccessKey)
			assert.Equal(t, credentials.SecretKey, test.expected.SecretKey)
		})
	}
}