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
	// These config values are likely to change semantics or go away
	// entirely between releases. Be careful when using them!
	Volatile struct {
		TLS struct {
			SkipPeerCAWhitelist bool
		}
	}
}

// Uplink represents the main entrypoint to Storj V3. An Uplink connects to
// a specific Satellite and caches connections and resources, allowing one to
// create sessions delineated by specific access controls.
type Uplink struct {
	tc transport.Client
}

// NewUplink creates a new Uplink
func NewUplink(ctx context.Context, cfg Config) (*Uplink, error) {
	identity, err := identity.NewFullIdentity(ctx, 0, 1)
	if err != nil {
		return nil, err
	}
	tlsOpts, err := tlsopts.NewOptions(identity, tlsopts.Config{UsePeerCAWhitelist: !cfg.Volatile.TLS.SkipPeerCAWhitelist})
	if err != nil {
		return nil, err
	}
	tc := transport.NewClient(tlsOpts)

	return &Uplink{
		tc: tc,
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

// Close closes the Uplink
func (u *Uplink) Close() error {
	return nil
}
