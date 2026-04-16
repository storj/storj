// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package secret

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultRefRegexp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string // full match strings
	}{
		{
			name:     "simple ref",
			input:    "ref+vault://secret/data/foo#/bar",
			expected: []string{"ref+vault://secret/data/foo#/bar"},
		},
		{
			name:     "ref with trailing plus",
			input:    "ref+vault://secret/data/foo#/bar+",
			expected: []string{"ref+vault://secret/data/foo#/bar+"},
		},
		{
			name:     "two refs with trailing plus in a string",
			input:    "foo ref+vault://s1#/a+ ref+vault://s2#/b+ bar",
			expected: []string{"ref+vault://s1#/a+", "ref+vault://s2#/b+"},
		},
		{
			name:     "secretref prefix",
			input:    "secretref+vault://secret/data/foo#/bar",
			expected: []string{"secretref+vault://secret/data/foo#/bar"},
		},
		{
			name:     "no match plain string",
			input:    "just a plain string",
			expected: nil,
		},
		{
			name:     "ref without fragment",
			input:    "ref+vault://secret/data/foo",
			expected: []string{"ref+vault://secret/data/foo"},
		},
		{
			name:     "ref with query params",
			input:    "ref+vault://secret/data/foo?version=2#/key",
			expected: []string{"ref+vault://secret/data/foo?version=2#/key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := DefaultRefRegexp.FindAllString(tt.input, -1)
			if tt.expected == nil {
				assert.Empty(t, matches)
			} else {
				assert.Equal(t, tt.expected, matches)
			}
		})
	}
}

func TestExtractFragment(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		fragment string
		expected string
		wantErr  bool
	}{
		{
			name:     "simple key",
			raw:      `{"password":"s3cret","username":"admin"}`,
			fragment: "/password",
			expected: "s3cret",
		},
		{
			name:     "nested key",
			raw:      `{"db":{"password":"s3cret"}}`,
			fragment: "/db/password",
			expected: "s3cret",
		},
		{
			name:     "numeric value marshaled",
			raw:      `{"port":5432}`,
			fragment: "/port",
			expected: "5432",
		},
		{
			name:     "missing key",
			raw:      `{"foo":"bar"}`,
			fragment: "/baz",
			wantErr:  true,
		},
		{
			name:     "invalid JSON",
			raw:      `not json`,
			fragment: "/key",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractFragment(tt.raw, tt.fragment)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestResolveToken(t *testing.T) {
	t.Run("from VAULT_TOKEN env", func(t *testing.T) {
		t.Setenv("VAULT_TOKEN", "my-token")
		t.Setenv("VAULT_TOKEN_FILE", "")
		token, err := resolveToken()
		require.NoError(t, err)
		assert.Equal(t, "my-token", token)
	})

	t.Run("from VAULT_TOKEN_FILE env", func(t *testing.T) {
		dir := t.TempDir()
		tokenFile := filepath.Join(dir, "token")
		require.NoError(t, os.WriteFile(tokenFile, []byte("file-token\n"), 0600))

		t.Setenv("VAULT_TOKEN", "")
		t.Setenv("VAULT_TOKEN_FILE", tokenFile)
		token, err := resolveToken()
		require.NoError(t, err)
		assert.Equal(t, "file-token", token)
	})

	t.Run("from ~/.vault-token", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HOME", dir)
		t.Setenv("VAULT_TOKEN", "")
		t.Setenv("VAULT_TOKEN_FILE", "")
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".vault-token"), []byte("home-token\n"), 0600))

		token, err := resolveToken()
		require.NoError(t, err)
		assert.Equal(t, "home-token", token)
	})

	t.Run("no token available", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("HOME", dir)
		t.Setenv("VAULT_TOKEN", "")
		t.Setenv("VAULT_TOKEN_FILE", "")
		// No .vault-token file in temp dir

		_, err := resolveToken()
		require.Error(t, err)
	})
}

// vaultKVv2Response builds a Vault KV v2 JSON response.
func vaultKVv2Response(data map[string]interface{}) []byte {
	resp := map[string]interface{}{
		"data": map[string]interface{}{
			"data":     data,
			"metadata": map[string]interface{}{},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestResolve(t *testing.T) {
	// Set up a fake Vault server.
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/secret/data/myapp", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Vault-Token") != "test-token" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		_, _ = w.Write(vaultKVv2Response(map[string]interface{}{
			"password": "s3cret",
			"username": "admin",
			"port":     5432,
		}))
	})
	mux.HandleFunc("/v1/secret/data/other", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(vaultKVv2Response(map[string]interface{}{
			"key": "otherval",
		}))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	t.Setenv("VAULT_ADDR", server.URL)
	t.Setenv("VAULT_TOKEN", "test-token")

	ctx := context.Background()

	t.Run("single ref with fragment", func(t *testing.T) {
		result, err := Resolve(ctx, "ref+vault://secret/data/myapp#/password")
		require.NoError(t, err)
		assert.Equal(t, "s3cret", result)
	})

	t.Run("ref embedded in text", func(t *testing.T) {
		result, err := Resolve(ctx, "user=ref+vault://secret/data/myapp#/username+ pass=ref+vault://secret/data/myapp#/password+")
		require.NoError(t, err)
		assert.Equal(t, "user=admin pass=s3cret", result)
	})

	t.Run("multiple refs from different paths", func(t *testing.T) {
		result, err := Resolve(ctx, "a=ref+vault://secret/data/myapp#/username+ b=ref+vault://secret/data/other#/key+")
		require.NoError(t, err)
		assert.Equal(t, "a=admin b=otherval", result)
	})

	t.Run("no refs returns input unchanged", func(t *testing.T) {
		result, err := Resolve(ctx, "just a plain string")
		require.NoError(t, err)
		assert.Equal(t, "just a plain string", result)
	})

	t.Run("ref without fragment returns full JSON", func(t *testing.T) {
		result, err := Resolve(ctx, "ref+vault://secret/data/myapp")
		require.NoError(t, err)
		var data map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(result), &data))
		assert.Equal(t, "s3cret", data["password"])
		assert.Equal(t, "admin", data["username"])
	})

	t.Run("unsupported backend", func(t *testing.T) {
		_, err := Resolve(ctx, "ref+aws://something")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported backend")
	})

	t.Run("missing secret path returns error", func(t *testing.T) {
		_, err := Resolve(ctx, "ref+vault://secret/data/nonexistent#/key")
		require.Error(t, err)
	})
}
