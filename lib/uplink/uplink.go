// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/metainfo/kvmetainfo"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/storage/buckets"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/storage/streams"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/uplink/metainfo"
)

var (
	maxBucketMetaSize = 10 * memory.MiB
)

// Config represents configuration options for an Uplink
type Config struct {
	// Volatile groups config values that are likely to change semantics
	// or go away entirely between releases. Be careful when using them!
	Volatile struct {
		// TLS defines options that affect TLS negotiation for outbound
		// connections initiated by this uplink.
		TLS struct {
			// SkipPeerCAWhitelist determines whether to require all
			// remote hosts to have identity certificates signed by
			// Certificate Authorities in the default whitelist. If
			// set to true, the whitelist will be ignored.
			SkipPeerCAWhitelist bool

			// PeerCAWhitelistPath gives the path to a CA cert
			// whitelist file. It is ignored if SkipPeerCAWhitelist
			// is set. If empty, the internal default peer whitelist
			// is used.
			PeerCAWhitelistPath string
		}

		// UseIdentity specifies the identity to be used by the uplink.
		// If nil, a new identity will be generated.
		UseIdentity *identity.FullIdentity

		// IdentityVersion is the identity version expected in a loaded
		// identity or used when creating an identity.
		IdentityVersion storj.IDVersion

		// PeerIDVersion is the identity versions remote peers to this node
		// will be supported by this node.
		PeerIDVersion string

		// MaxInlineSize determines whether the uplink will attempt to
		// store a new object in the satellite's pointerDB. Objects at
		// or below this size will be marked for inline storage, and
		// objects above this size will not. (The satellite may reject
		// the inline storage and require remote storage, still.)
		MaxInlineSize memory.Size
	}
}

func (c *Config) setDefaults(ctx context.Context) error {
	if c.Volatile.UseIdentity == nil {
		var err error
		c.Volatile.UseIdentity, err = identity.NewFullIdentity(ctx, identity.NewCAOptions{
			VersionNumber: c.Volatile.IdentityVersion.Number,
			Difficulty:    0,
			Concurrency:   1,
		})
		if err != nil {
			return err
		}
	}
	idVersion, err := c.Volatile.UseIdentity.Version()
	if err != nil {
		return err
	}
	if idVersion.Number != c.Volatile.IdentityVersion.Number {
		return storj.ErrVersion.New("`UseIdentity` version (%d) didn't match version in config (%d)", idVersion.Number, c.Volatile.IdentityVersion.Number)
	}
	if c.Volatile.MaxInlineSize.Int() == 0 {
		c.Volatile.MaxInlineSize = 4 * memory.KiB
	}
	return nil
}

// Uplink represents the main entrypoint to Storj V3. An Uplink connects to
// a specific Satellite and caches connections and resources, allowing one to
// create sessions delineated by specific access controls.
type Uplink struct {
	tc  transport.Client
	cfg *Config
}

// NewUplink creates a new Uplink
func NewUplink(ctx context.Context, cfg *Config) (*Uplink, error) {
	if cfg == nil {
		cfg = &Config{}
	}
	if err := cfg.setDefaults(ctx); err != nil {
		return nil, err
	}
	tlsConfig := tlsopts.Config{
		UsePeerCAWhitelist:  !cfg.Volatile.TLS.SkipPeerCAWhitelist,
		PeerCAWhitelistPath: cfg.Volatile.TLS.PeerCAWhitelistPath,
		PeerIDVersions:      cfg.Volatile.PeerIDVersion,
	}
	tlsOpts, err := tlsopts.NewOptions(cfg.Volatile.UseIdentity, tlsConfig)
	if err != nil {
		return nil, err
	}
	tc := transport.NewClient(tlsOpts)

	return &Uplink{
		tc:  tc,
		cfg: cfg,
	}, nil
}

// OpenProject returns a Project handle with the given APIKey
func (u *Uplink) OpenProject(ctx context.Context, satelliteAddr string, apiKey APIKey) (p *Project, err error) {
	defer mon.Task()(&ctx)(&err)

	metainfo, err := metainfo.NewClient(ctx, u.tc, satelliteAddr, apiKey.key)
	if err != nil {
		return nil, err
	}

	// TODO: we shouldn't need segment or stream stores to manage buckets
	segments := segments.NewSegmentStore(metainfo, nil, eestream.RedundancyStrategy{}, maxBucketMetaSize.Int(), maxBucketMetaSize.Int64())
	streams, err := streams.NewStreamStore(segments, maxBucketMetaSize.Int64(), nil, 0, storj.Unencrypted)
	if err != nil {
		return nil, err
	}

	return &Project{
		tc:            u.tc,
		metainfo:      metainfo,
		project:       kvmetainfo.NewProject(buckets.NewStore(streams)),
		maxInlineSize: u.cfg.Volatile.MaxInlineSize,
	}, nil
}

// Close closes the Uplink. This may not do anything at present, but should
// still be called to allow forward compatibility.
func (u *Uplink) Close() error {
	return nil
}
