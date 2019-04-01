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

			PeerCAWhitelistPath string
		}

		UseIdentity *identity.FullIdentity
	}
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
	id := cfg.Volatile.UseIdentity
	if id == nil {
		var err error
		id, err = identity.NewFullIdentity(ctx, 0, 1)
		if err != nil {
			return nil, err
		}
	}
	tlsConfig := tlsopts.Config{
		UsePeerCAWhitelist:  !cfg.Volatile.TLS.SkipPeerCAWhitelist,
		PeerCAWhitelistPath: cfg.Volatile.TLS.PeerCAWhitelistPath,
	}
	tlsOpts, err := tlsopts.NewOptions(id, tlsConfig)
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
		tc:       u.tc,
		metainfo: metainfo,
		project:  kvmetainfo.NewProject(buckets.NewStore(streams)),
	}, nil
}

// Close closes the Uplink. This may not do anything at present, but should
// still be called to allow forward compatibility.
func (u *Uplink) Close() error {
	return nil
}
