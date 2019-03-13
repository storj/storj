// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"fmt"

	"storj.io/storj/pkg/transport"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/metainfo/kvmetainfo"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/storj"
)

// Config holds the configs for the Uplink
// type Config struct {
// 	// MaxBufferMem controls upload performance and is system-specific
// 	MaxBufferMem int

// 	// These should only be relevant for new files; these values for existing
// 	// files should come from the metainfo index. It's unlikely these will ever
// 	// change much.
// 	EncBlockSize  int
// 	MaxInlineSize int
// 	SegmentSize   int64
// 	TLSConfig     tlsopts.Config

// 	// Default Configs
// 	SatelliteAddr string
// 	APIKey        string
// }

// Session represents a specific access session.
type Session struct {
	transport transport.Client
	client    *Client
}

// Access is all of the access information an application needs to store and
// retrieve data. Someone with a share may have no restrictions within a project
// (can create buckets, list buckets, list files, upload files, delete files,
// etc), may be restricted to a single bucket, may be restricted to a prefix
// within a bucket, or may even be restricted to a single file within a bucket.
// NB(dylan): You need an Access to start a Session
type Access struct {
	Permissions Macaroon

	// TODO: these should be per-bucket somehow maybe? oh man what a nightmare
	// Could be done via []Bucket with struct that has each of these
	// PathCipher       storj.Cipher // i.e. storj.AESGCM
	// EncPathPrefix    storj.Path
	// Key              storj.Key
	// EncryptionScheme storj.EncryptionScheme

	// Something like this?
	// TODO(dylan): Shouldn't actually use string, this is just a placeholder
	// until a more precise type is figured out - probably type Bucket
	Buckets map[string]BucketOpts
}

// Uplink represents the main entrypoint to Storj V3. An Uplink connects to
// a specific Satellite and caches connections and resources, allowing one to
// create sessions delineated by specific access controls.
type Uplink struct {
	id            *identity.FullIdentity
	session       map[string]*Session
	satelliteAddr string
	config        Config
}

// NewUplink creates a new Uplink
func NewUplink(ident *identity.FullIdentity, satelliteAddr string, cfg Config) *Uplink {
	return &Uplink{
		id:            ident,
		satelliteAddr: satelliteAddr,
		config:        cfg,
	}
}

// NewSession creates a Session with an Access struct.
func (u *Uplink) NewSession(ctx context.Context, bucketName string, cfg Config, access Access) error {
	opts, err := tlsopts.NewOptions(u.id, u.config.TLS)
	if err != nil {
		fmt.Printf("tlsopts error: %+v\n", err)
		return err
	}

	tc := transport.NewClient(opts)
	gateway, err := u.NewClient(ctx, u.id, cfg)
	if err != nil {
		fmt.Printf("gateway error: %+v\n", err)
		return err
	}

	// TODO: Only handling this via name until I get a better alternative implemented.
	u.session = make(map[string]*Session)

	u.session[bucketName] = &Session{
		transport: tc,
		client:    gateway,
	}

	return nil
}

// Access creates a new share, potentially further restricted from the Access used
// to create this session.
func (s *Session) Access(ctx context.Context, caveats ...Caveat) (Access, error) {
	panic("TODO")
}

// NewClient returns a gateway instance
func (u *Uplink) NewClient(ctx context.Context, identity *identity.FullIdentity, cfg Config) (*Client, error) {
	// TODO: (dylan) Need to merge these defaults with Configs from this library
	// TODO: (dylan) Need to allow users of library to set defaults easier as well
	metainfo, streams, err := cfg.GetMetainfo(ctx, identity)
	if err != nil {
		return nil, err
	}

	client := NewStorjUplink(
		metainfo,
		streams,
		storj.AESGCM,
		kvmetainfo.DefaultES,
		kvmetainfo.DefaultRS,
	)

	return client, nil
}
