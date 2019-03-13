// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"github.com/zeebo/errs"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
)

/*
* libuplink TODOS (dylan):
- [ ] Remove all references to github.com/minio/minio
- [ ] Make this package standalone so that pkg/miniogw can wrap it for minio rather than the other way around.
- [ ] Clean up naming conventions
- [ ] Sort functions into files & general cleanup
*/

var (
	mon = monkit.Package()

	// Error is the errs class of standard End User Client errors
	Error = errs.Class("libuplink error")
)

// NewStorjUplink creates a *Storj object from an existing ObjectStore
func NewStorjUplink(metainfo storj.Metainfo, streams streams.Store, pathCipher storj.Cipher, encryption storj.EncryptionScheme, redundancy storj.RedundancyScheme) *Client {
	return &Client{
		metainfo:   metainfo,
		streams:    streams,
		pathCipher: pathCipher,
		encryption: encryption,
		redundancy: redundancy,
		multipart:  NewMultipartUploads(),
	}
}

// Client is the implementation of a minio cmd.Gateway
type Client struct {
	metainfo   storj.Metainfo
	streams    streams.Store
	pathCipher storj.Cipher
	encryption storj.EncryptionScheme
	redundancy storj.RedundancyScheme
	multipart  *MultipartUploads
}

// Name implements cmd.Gateway
func (client *Client) Name() string {
	return "storj"
}

// // NewGatewayLayer implements cmd.Gateway
// func (client *Client) NewGatewayLayer(creds auth.Credentials) (minio.ObjectLayer, error) {
// 	return &gatewayLayer{gateway: client}, nil
// }

// Production implements cmd.Gateway
func (client *Client) Production() bool {
	return false
}

// type gatewayLayer struct {
// 	// minio.GatewayUnsupported
// 	gateway *Client
// }
