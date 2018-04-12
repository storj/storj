// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"os"
)

// DefaultURL of the Storj Bridge API endpoint
const DefaultURL = "https://api.storj.io"

// Env contains parameters for accessing the Storj network
type Env struct {
	URL      string
	User     string
	Password string
	Mnemonic string
}

// NewEnv creates new Env struct with default values
func NewEnv() Env {
	return Env{
		URL:      DefaultURL,
		User:     os.Getenv("STORJ_BRIDGE_USER"),
		Password: sha256Sum(os.Getenv("STORJ_BRIDGE_PASS")),
		Mnemonic: os.Getenv("STORJ_ENCRYPTION_KEY"),
	}
}
