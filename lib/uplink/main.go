// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"

	minio "github.com/minio/minio/cmd"
	"storj.io/storj/pkg/transport"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls/tlsopts"
)

// Config holds the configs for the Uplink
type Config struct {
	// MaxBufferMem controls upload performance and is system-specific
	MaxBufferMem int

	// These should only be relevant for new files; these values for existing
	// files should come from the metainfo index. It's unlikely these will ever
	// change much.
	EncBlockSize  int
	MaxInlineSize int
	SegmentSize   int64
	TLSConfig     tlsopts.Config
}

// Uplink represents the main entrypoint to Storj V3. An Uplink connects to
// a specific Satellite and caches connections and resources, allowing one to
// create sessions delineated by specific access controls.
type Uplink struct {
	ID            *identity.FullIdentity
	Session       *Session
	SatelliteAddr string
	Config        Config
}

// NewUplink creates a new Uplink
func NewUplink(ident *identity.FullIdentity, satelliteAddr string, cfg Config) *Uplink {
	return &Uplink{
		ID:            ident,
		SatelliteAddr: satelliteAddr,
		Config:        cfg,
	}
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

// ParseAccess parses a serialized Access
func ParseAccess(data []byte) (Access, error) {
	panic("TODO")
}

// Serialize serializes an Access message
func (a *Access) Serialize() ([]byte, error) {
	panic("TODO")
}

// Session represents a specific access session.
type Session struct {
	TransportClient transport.Client
	Gateway         *minio.ObjectLayer
}

// NewSession creates a Session with an Access struct.
func (u *Uplink) NewSession(access Access) error {
	opts, err := tlsopts.NewOptions(u.ID, u.Config.TLSConfig)
	if err != nil {
		return err
	}

	tc := transport.NewClient(opts)

	// gateway := miniogw.

	u.Session = &Session{
		TransportClient: tc,
		Gateway:         nil,
	}

	return nil
}

// Access creates a new share, potentially further restricted from the Access used
// to create this session.
func (s *Session) Access(ctx context.Context, caveats ...Caveat) (Access, error) {
	panic("TODO")
}
