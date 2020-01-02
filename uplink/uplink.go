// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/storj/uplink/metainfo"
	"storj.io/storj/uplink/metainfo/kvmetainfo"
)

const defaultUplinkDialTimeout = 20 * time.Second

// Config represents configuration options for an Uplink
type Config struct {
	// Volatile groups config values that are likely to change semantics
	// or go away entirely between releases. Be careful when using them!
	Volatile struct {
		// Log is the logger to use for uplink components
		Log *zap.Logger

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

		// PeerIDVersion is the identity versions remote peers to this node
		// will be supported by this node.
		PeerIDVersion string

		// MaxInlineSize determines whether the uplink will attempt to
		// store a new object in the satellite's metainfo. Objects at
		// or below this size will be marked for inline storage, and
		// objects above this size will not. (The satellite may reject
		// the inline storage and require remote storage, still.)
		MaxInlineSize memory.Size

		// MaxMemory is the default maximum amount of memory to be
		// allocated for read buffers while performing decodes of
		// objects. (This option is overrideable per Bucket if the user
		// so desires.) If set to zero, the library default (4 MiB) will
		// be used. If set to a negative value, the system will use the
		// smallest amount of memory it can.
		MaxMemory memory.Size

		// PartnerID is the identity given to the partner for value
		// attribution.
		//
		// Deprecated: prefer UserAgent
		PartnerID string

		// UserAgent for the product using the library.
		UserAgent string

		// DialTimeout is the maximum time to wait connecting to another node.
		// If not set, the library default (20 seconds) will be used.
		DialTimeout time.Duration

		// PBKDFConcurrency is the passphrase-based key derivation function
		// concurrency to use.
		// WARNING: changing this value fundamentally changes how keys are
		// derived. Keys generated with one value will not be the same keys
		// as generated with other values! Leaving this at the default is
		// highly recommended.
		//
		// Unfortunately, up to version v0.26.2, we automatically set this to the
		// number of CPU cores your processor had. If you are having trouble
		// decrypting data uploaded with v0.26.2 or older, you may need to set
		// this value to the number of cores your computer had at the time
		// you entered a passphrase.
		//
		// Otherwise, this value should be left at the default value of 0
		// (which means to use the internal default).
		PBKDFConcurrency int
	}
}

func (cfg *Config) clone() *Config {
	clone := *cfg
	return &clone
}

func (cfg *Config) setDefaults(ctx context.Context) error {
	if cfg.Volatile.MaxInlineSize == 0 {
		cfg.Volatile.MaxInlineSize = 4 * memory.KiB
	}
	if cfg.Volatile.MaxMemory.Int() == 0 {
		cfg.Volatile.MaxMemory = 4 * memory.MiB
	} else if cfg.Volatile.MaxMemory.Int() < 0 {
		cfg.Volatile.MaxMemory = 0
	}
	if cfg.Volatile.Log == nil {
		cfg.Volatile.Log = zap.NewNop()
	}
	if cfg.Volatile.DialTimeout.Seconds() == 0 {
		cfg.Volatile.DialTimeout = defaultUplinkDialTimeout
	}
	if cfg.Volatile.PBKDFConcurrency == 0 {
		// WARNING: if this default value changes, the root keys of every user will change.
		// So, don't change this without sufficiently good reason.
		// some other argon2 wrapper libraries have chosen 8 as the default, so
		// we do here.
		cfg.Volatile.PBKDFConcurrency = 8
	}
	if cfg.Volatile.PBKDFConcurrency < 0 || cfg.Volatile.PBKDFConcurrency >= 256 {
		return errs.New("Invalid value for PBKDFConcurrency (must fit in a uint8)")
	}
	return nil
}

// Uplink represents the main entrypoint to Storj V3. An Uplink connects to
// a specific Satellite and caches connections and resources, allowing one to
// create sessions delineated by specific access controls.
type Uplink struct {
	ident  *identity.FullIdentity
	dialer rpc.Dialer
	cfg    *Config
}

// NewUplink creates a new Uplink. This is the first step to create an uplink
// session with a user specified config or with default config, if nil config
func NewUplink(ctx context.Context, cfg *Config) (_ *Uplink, err error) {
	defer mon.Task()(&ctx)(&err)

	ident, err := identity.NewFullIdentity(ctx, identity.NewCAOptions{
		Difficulty:  9,
		Concurrency: 1,
	})
	if err != nil {
		return nil, err
	}

	if cfg == nil {
		cfg = &Config{}
	}
	cfg = cfg.clone()
	if err := cfg.setDefaults(ctx); err != nil {
		return nil, err
	}
	tlsConfig := tlsopts.Config{
		UsePeerCAWhitelist:  !cfg.Volatile.TLS.SkipPeerCAWhitelist,
		PeerCAWhitelistPath: cfg.Volatile.TLS.PeerCAWhitelistPath,
		PeerIDVersions:      "0",
	}

	tlsOptions, err := tlsopts.NewOptions(ident, tlsConfig, nil)
	if err != nil {
		return nil, err
	}

	dialer := rpc.NewDefaultDialer(tlsOptions)
	dialer.DialTimeout = cfg.Volatile.DialTimeout

	return &Uplink{
		ident:  ident,
		dialer: dialer,
		cfg:    cfg,
	}, nil
}

// TODO: move the project related OpenProject and Close to project.go

// OpenProject returns a Project handle with the given APIKey
func (u *Uplink) OpenProject(ctx context.Context, satelliteAddr string, apiKey APIKey) (p *Project, err error) {
	defer mon.Task()(&ctx)(&err)

	m, err := metainfo.Dial(ctx, u.dialer, satelliteAddr, apiKey.key, u.cfg.Volatile.UserAgent)
	if err != nil {
		return nil, err
	}

	project, err := kvmetainfo.SetupProject(m)
	if err != nil {
		return nil, err
	}

	return &Project{
		uplinkCfg: u.cfg,
		dialer:    u.dialer,
		metainfo:  m,
		project:   project,
	}, nil
}

// Close closes the Project. Opened buckets or objects must not be used after calling Close.
func (p *Project) Close() error {
	return p.metainfo.Close()
}

// Close closes the Uplink. Opened projects, buckets or objects must not be used after calling Close.
func (u *Uplink) Close() error {
	return nil
}
