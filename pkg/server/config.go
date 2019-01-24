// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package server

import (
	"storj.io/storj/pkg/peertls"
)

// Config holds server specific configuration parameters
type Config struct {
	RevocationDBURL     string `help:"url for revocation database (e.g. bolt://some.db OR redis://127.0.0.1:6378?db=2&password=abc123)" default:"bolt://$CONFDIR/revocations.db"`
	PeerCAWhitelistPath string `help:"path to the CA cert whitelist (peer identities must be signed by one these to be verified). this will override the default peer whitelist"`
	UsePeerCAWhitelist  bool   `help:"if true, uses peer ca whitelist checking" default:"false"`
	Address             string `user:"true" help:"address to listen on" default:":7777"`
	Extensions          peertls.TLSExtConfig
}
