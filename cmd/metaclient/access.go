// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"fmt"
	"os"

	"github.com/zeebo/clingy"
)

const (
	STORJ_METASEARCH_SERVER_ENV = "STORJ_METASEARCH_SERVER"
	STORJ_METASEARCH_ACCESS_ENV = "STORJ_METASEARCH_ACCESS"
)

// AccessOptions contains the URL and credentials for the metasearch service.
type AccessOptions struct {
	Access string
	Server string
}

func newAccessOptions() *AccessOptions {
	return &AccessOptions{}
}

func (a *AccessOptions) Setup(params clingy.Parameters) {
	a.Access = params.Flag("access", "Access Key (default: STORJ_METASEARCH_ACCESS env variable)", "").(string)
	a.Server = params.Flag("server", "Metasearch Server (default: STORJ_METASEARCH_SERVER env variable)", "").(string)
}

func (a *AccessOptions) Validate() error {
	if a.Server == "" {
		a.Server = os.Getenv(STORJ_METASEARCH_SERVER_ENV)
	}
	if a.Access == "" {
		a.Access = os.Getenv(STORJ_METASEARCH_ACCESS_ENV)
	}

	if a.Server == "" {
		return fmt.Errorf("missing server URL. Pass --server or set %s", STORJ_METASEARCH_SERVER_ENV)
	}
	if a.Access == "" {
		return fmt.Errorf("missing access key. Pass --access or set %s", STORJ_METASEARCH_ACCESS_ENV)
	}

	return nil
}
