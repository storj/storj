// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"storj.io/common/version"
)

var (
	mon = monkit.Package()

	// Error is the error class for version checker client errors.
	Error = errs.Class("version checker client")
)

// ClientConfig is the config struct for the version control client.
type ClientConfig struct {
	ServerAddress  string        `help:"server address to check its version against" default:"https://version.storj.io"`
	RequestTimeout time.Duration `help:"Request timeout for version checks" default:"0h1m0s"`
}

// Client defines helper methods for using version control server response data.
//
// architecture: Client
type Client struct {
	config ClientConfig
}

// New constructs a new verson control server client.
func New(config ClientConfig) *Client {
	return &Client{
		config: config,
	}
}

// All handles the HTTP request to gather the latest version information.
func (client *Client) All(ctx context.Context) (ver version.AllowedVersions, err error) {
	defer mon.Task()(&ctx)(&err)

	// Tune Client to have a custom Timeout (reduces hanging software)
	httpClient := http.Client{
		Timeout: client.config.RequestTimeout,
	}

	// New Request that used the passed in context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, client.config.ServerAddress, nil)
	if err != nil {
		return version.AllowedVersions{}, Error.Wrap(err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return version.AllowedVersions{}, Error.Wrap(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return version.AllowedVersions{}, Error.Wrap(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return version.AllowedVersions{}, Error.New("non-success http status '%s' code: %d; body: %s\n", client.config.ServerAddress, resp.StatusCode, body)
	}

	err = json.NewDecoder(bytes.NewReader(body)).Decode(&ver)
	return ver, Error.Wrap(err)
}

// Process returns the version info for the named process from the version control server response.
func (client *Client) Process(ctx context.Context, processName string) (process version.Process, err error) {
	defer mon.Task()(&ctx, processName)(&err)

	versions, err := client.All(ctx)
	if err != nil {
		return version.Process{}, Error.Wrap(err)
	}

	processesValue := reflect.ValueOf(versions.Processes)
	field := processesValue.FieldByName(kebabToPascal(processName))

	processNameErr := Error.New("invalid process name: %s\n", processName)
	if field == (reflect.Value{}) {
		return version.Process{}, processNameErr
	}

	process, ok := field.Interface().(version.Process)
	if !ok {
		return version.Process{}, processNameErr
	}

	return process, nil
}

// kebabToPascal converts `alpha-beta` to `AlphaBeta`.
func kebabToPascal(str string) string {
	return strings.ReplaceAll(cases.Title(language.Und, cases.NoLower).String(str), "-", "")
}
