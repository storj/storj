// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

// Package ulext provides an interface for the CLI to interface with the external world.
package ulext

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/rpc/rpcpool"
	"storj.io/storj/cmd/uplink/ulfs"
	"storj.io/uplink"
	"storj.io/uplink/private/testuplink"
)

// External is the interface for all of the ways that the uplink command may interact with
// any external state.
type External interface {
	OpenFilesystem(ctx context.Context, accessName string, options ...Option) (ulfs.Filesystem, error)
	OpenProject(ctx context.Context, accessName string, options ...Option) (*uplink.Project, error)
	GetEdgeUrlOverrides(ctx context.Context, access *uplink.Access) (EdgeURLOverrides, error)

	AccessInfoFile() (path string, err error)
	OpenAccess(accessName string) (access *uplink.Access, err error)
	GetAccessInfo(required bool) (defaultName string, accesses map[string]string, err error)
	SaveAccessInfo(defaultName string, accesses map[string]string) error
	RequestAccess(ctx context.Context, satelliteAddress, apiKey, passphrase string, unencryptedObjectKeys bool) (*uplink.Access, error)
	ExportAccess(ctx context.Context, access *uplink.Access, filename string) error

	ConfigFile() (path string, err error)
	SaveConfig(values map[string]string) error

	PromptInput(ctx context.Context, prompt string) (input string, err error)
	PromptSecret(ctx context.Context, prompt string) (secret string, err error)
}

// EdgeURLOverrides contains the URL overrides for the edge service.
type EdgeURLOverrides struct {
	AuthService         string
	PublicLinksharing   string
	InternalLinksharing string
}

// Options contains all of the possible options for opening a filesystem or project.
type Options struct {
	EncryptionBypass               bool
	ConnectionPoolOptions          rpcpool.Options
	ConcurrentSegmentUploadsConfig testuplink.ConcurrentSegmentUploadsConfig
}

// LoadOptions takes a slice of Option values and returns a filled out Options struct.
func LoadOptions(options ...Option) (opts Options) {
	for _, opt := range options {
		opt.apply(&opts)
	}
	return opts
}

// Option is a single option that controls the Options struct.
type Option struct {
	apply func(*Options)
}

// BypassEncryption will disable decrypting of path names if bypass is true.
func BypassEncryption(bypass bool) Option {
	return Option{apply: func(opt *Options) { opt.EncryptionBypass = bypass }}
}

// ConnectionPoolOptions will initialize the connection pool with options.
func ConnectionPoolOptions(options rpcpool.Options) Option {
	return Option{apply: func(opt *Options) { opt.ConnectionPoolOptions = options }}
}

// ConcurrentSegmentUploadsConfig will initialize the concurrent segment uploads config with config.
func ConcurrentSegmentUploadsConfig(config testuplink.ConcurrentSegmentUploadsConfig) Option {
	return Option{apply: func(opt *Options) { opt.ConcurrentSegmentUploadsConfig = config }}
}

// RegisterAccess registers an access grant with a Gateway Authorization Service.
func RegisterAccess(ctx context.Context, access *uplink.Access, authService string, public bool, timeout time.Duration) (accessKey, secretKey, endpoint string, err error) {
	if authService == "" {
		return "", "", "", errs.New("no auth service address provided")
	}
	accessSerialized, err := access.Serialize()
	if err != nil {
		return "", "", "", errs.Wrap(err)
	}
	postData, err := json.Marshal(map[string]interface{}{
		"access_grant": accessSerialized,
		"public":       public,
	})
	if err != nil {
		return accessKey, "", "", errs.Wrap(err)
	}

	client := &http.Client{
		Timeout: timeout,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, authService+"/v1/access", bytes.NewReader(postData))
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", err
	}

	respBody := make(map[string]string)
	if err := json.Unmarshal(body, &respBody); err != nil {
		return "", "", "", errs.New("unexpected response from auth service: %s", string(body))
	}

	accessKey, ok := respBody["access_key_id"]
	if !ok {
		return "", "", "", errs.New("access_key_id missing in response")
	}
	secretKey, ok = respBody["secret_key"]
	if !ok {
		return "", "", "", errs.New("secret_key missing in response")
	}
	return accessKey, secretKey, respBody["endpoint"], nil
}
