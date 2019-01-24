// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb

// Config contains configuration for console web server
type Config struct {
	Address   string `help:"server address of the graphql api gateway and frontend app" default:"127.0.0.1:8081"`
	StaticDir string `help:"path to static resources" default:""`
}
