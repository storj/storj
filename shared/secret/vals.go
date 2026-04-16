// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package secret

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultRefRegexp matches ref+BACKEND://PATH[?PARAMS][#FRAGMENT][+] expressions.
// Compatible with helmfile/vals pattern.
var DefaultRefRegexp = regexp.MustCompile(`((secret)?ref)\+([^\+:]*://[^\+\n ]+[^\+\n ",])\+?`)

// Resolve replaces all ref+vault://... expressions in the input string
// with secret values fetched from Vault/OpenBao.
//
// The supported URI format is:
//
//	ref+vault://PATH[#FRAGMENT][+]
//
// PATH is the Vault secret path (e.g. secret/data/myapp).
// FRAGMENT is a JSON/YAML key path to extract a single field (e.g. #/password).
// A trailing + explicitly ends the expression, useful for inline interpolation.
//
// Vault address is read from VAULT_ADDR env.
// Token is resolved from VAULT_TOKEN env, then VAULT_TOKEN_FILE env, then ~/.vault-token.
func Resolve(ctx context.Context, input string) (string, error) {
	matches := DefaultRefRegexp.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 {
		return input, nil
	}

	client, err := newVaultClient()
	if err != nil {
		return "", err
	}

	var result strings.Builder
	lastEnd := 0

	for _, match := range matches {
		// match[0]:match[1] is the full match
		// match[6]:match[7] is capture group 3 — the URI part after ref+
		fullStart, fullEnd := match[0], match[1]
		uriStart, uriEnd := match[6], match[7]

		result.WriteString(input[lastEnd:fullStart])

		uri := input[uriStart:uriEnd]
		val, err := resolveURI(ctx, client, uri)
		if err != nil {
			return "", fmt.Errorf("resolving %q: %w", uri, err)
		}
		result.WriteString(val)

		lastEnd = fullEnd
	}

	result.WriteString(input[lastEnd:])
	return result.String(), nil
}

func resolveURI(ctx context.Context, client *vaultClient, rawURI string) (string, error) {
	parsed, err := url.Parse(rawURI)
	if err != nil {
		return "", fmt.Errorf("parsing URI %q: %w", rawURI, err)
	}

	if parsed.Scheme != "vault" {
		return "", fmt.Errorf("unsupported backend %q (only \"vault\" is supported)", parsed.Scheme)
	}

	// The path from the URI. url.Parse puts the host in Host and rest in Path
	// for vault://secret/data/foo, Host="secret", Path="/data/foo"
	secretPath := parsed.Host + parsed.Path

	raw, err := client.read(ctx, secretPath)
	if err != nil {
		return "", err
	}

	fragment := parsed.Fragment
	if fragment == "" {
		return raw, nil
	}

	return extractFragment(raw, fragment)
}

// extractFragment parses the raw value as JSON and traverses
// the path specified by fragment to extract a single value.
// Fragment is a /-separated path like "/password" or "/nested/key".
func extractFragment(raw string, fragment string) (string, error) {
	var data interface{}
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return "", fmt.Errorf("parsing secret as JSON for fragment extraction: %w", err)
	}

	parts := strings.Split(strings.TrimPrefix(fragment, "/"), "/")
	current := data
	for _, part := range parts {
		if part == "" {
			continue
		}
		m, ok := current.(map[string]interface{})
		if !ok {
			return "", fmt.Errorf("cannot traverse fragment %q: value at %q is not an object", fragment, part)
		}
		current, ok = m[part]
		if !ok {
			return "", fmt.Errorf("fragment %q: key %q not found", fragment, part)
		}
	}

	switch v := current.(type) {
	case string:
		return v, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("marshaling fragment value: %w", err)
		}
		return string(b), nil
	}
}

type vaultClient struct {
	addr  string
	token string
	http  *http.Client
}

func newVaultClient() (*vaultClient, error) {
	addr := os.Getenv("VAULT_ADDR")
	if addr == "" {
		return nil, fmt.Errorf("VAULT_ADDR environment variable is not set")
	}

	token, err := resolveToken()
	if err != nil {
		return nil, err
	}

	return &vaultClient{
		addr:  strings.TrimRight(addr, "/"),
		token: token,
		http:  &http.Client{},
	}, nil
}

func resolveToken() (string, error) {
	if token := os.Getenv("VAULT_TOKEN"); token != "" {
		return token, nil
	}

	if tokenFile := os.Getenv("VAULT_TOKEN_FILE"); tokenFile != "" {
		data, err := os.ReadFile(tokenFile)
		if err != nil {
			return "", fmt.Errorf("reading VAULT_TOKEN_FILE %q: %w", tokenFile, err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".vault-token"))
	if err != nil {
		return "", fmt.Errorf("no vault token found: set VAULT_TOKEN, VAULT_TOKEN_FILE, or create ~/.vault-token")
	}
	return strings.TrimSpace(string(data)), nil
}

// read fetches a secret from the Vault HTTP API (v1).
// For KV v2 secrets the path should include /data/ (e.g. secret/data/myapp).
func (c *vaultClient) read(ctx context.Context, path string) (string, error) {
	reqURL := c.addr + "/v1/" + path

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("X-Vault-Token", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault request to %s: %w", reqURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading vault response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vault returned %d for %s: %s", resp.StatusCode, path, string(body))
	}

	// Parse the Vault response envelope.
	// KV v2 response: {"data": {"data": {...}, "metadata": {...}}}
	// KV v1 response: {"data": {...}}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return "", fmt.Errorf("parsing vault response: %w", err)
	}

	// Check if this is a KV v2 response (has nested data.data).
	var kvV2 struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(envelope.Data, &kvV2); err == nil && kvV2.Data != nil {
		return string(kvV2.Data), nil
	}

	return string(envelope.Data), nil
}
