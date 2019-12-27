// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package mobile

import (
	"fmt"

	"storj.io/common/memory"
	"storj.io/common/storj"
	libuplink "storj.io/storj/lib/uplink"
)

// Config represents configuration options for an Uplink
type Config struct {

	// MaxInlineSize determines whether the uplink will attempt to
	// store a new object in the satellite's metainfo. Objects at
	// or below this size will be marked for inline storage, and
	// objects above this size will not. (The satellite may reject
	// the inline storage and require remote storage, still.)
	MaxInlineSize int64

	// MaxMemory is the default maximum amount of memory to be
	// allocated for read buffers while performing decodes of
	// objects. (This option is overrideable per Bucket if the user
	// so desires.) If set to zero, the library default (4 MiB) will
	// be used. If set to a negative value, the system will use the
	// smallest amount of memory it can.
	MaxMemory int64

	// SkipPeerCAWhitelist determines whether to require all
	// remote hosts to have identity certificates signed by
	// Certificate Authorities in the default whitelist. If
	// set to true, the whitelist will be ignored.
	SkipPeerCAWhitelist bool
}

// Uplink represents the main entrypoint to Storj V3. An Uplink connects to
// a specific Satellite and caches connections and resources, allowing one to
// create sessions delineated by specific access controls.
type Uplink struct {
	scope
	lib *libuplink.Uplink
}

// NewUplink creates a new Uplink. This is the first step to create an uplink
// session with a user specified config or with default config, if nil config.
// Uplink needs also writable temporary directory.
func NewUplink(config *Config, tempDir string) (*Uplink, error) {
	scope := rootScope(tempDir)

	cfg := &libuplink.Config{}
	if config != nil {
		// TODO: V3-2303, support logging somehow
		cfg.Volatile.TLS.SkipPeerCAWhitelist = config.SkipPeerCAWhitelist
		cfg.Volatile.MaxInlineSize = memory.Size(config.MaxInlineSize)
		cfg.Volatile.MaxMemory = memory.Size(config.MaxMemory)
	}

	lib, err := libuplink.NewUplink(scope.ctx, cfg)
	if err != nil {
		return nil, safeError(err)
	}
	return &Uplink{scope, lib}, nil
}

// Close closes the Uplink. This may not do anything at present, but should
// still be called to allow forward compatibility. No Project or Bucket
// objects using this Uplink should be used after calling Close.
func (uplink *Uplink) Close() error {
	uplink.cancel()
	return safeError(uplink.lib.Close())
}

// Project represents a specific project access session.
type Project struct {
	scope
	lib *libuplink.Project
}

// OpenProject returns a Project handle with the given APIKey
func (uplink *Uplink) OpenProject(satellite string, apikey string) (*Project, error) {
	scope := uplink.scope.child()

	key, err := libuplink.ParseAPIKey(apikey)
	if err != nil {
		return nil, safeError(err)
	}

	project, err := uplink.lib.OpenProject(scope.ctx, satellite, key)
	if err != nil {
		return nil, safeError(err)
	}

	return &Project{scope, project}, nil
}

// Close closes the Project
func (project *Project) Close() error {
	defer project.cancel()
	return safeError(project.lib.Close())
}

// CreateBucket creates buckets in project
func (project *Project) CreateBucket(bucketName string, opts *BucketConfig) (*BucketInfo, error) {
	scope := project.scope.child()

	cfg := libuplink.BucketConfig{}
	if opts != nil {
		cfg.PathCipher = storj.CipherSuite(opts.PathCipher)
		cfg.EncryptionParameters = newStorjEncryptionParameters(opts.EncryptionParameters)
		cfg.Volatile.RedundancyScheme = newStorjRedundancyScheme(opts.RedundancyScheme)
		cfg.Volatile.SegmentsSize = memory.Size(opts.SegmentsSize)
	}

	bucket, err := project.lib.CreateBucket(scope.ctx, bucketName, &cfg)
	if err != nil {
		return nil, safeError(err)
	}

	return newBucketInfo(bucket), nil
}

// OpenBucket returns a Bucket handle with the given EncryptionAccess
// information.
func (project *Project) OpenBucket(bucketName string, access *EncryptionAccess) (*Bucket, error) {
	scope := project.scope.child()

	bucket, err := project.lib.OpenBucket(scope.ctx, bucketName, access.lib)
	if err != nil {
		return nil, safeError(err)
	}

	return &Bucket{bucket.Name, scope, bucket}, nil
}

// GetBucketInfo returns info about the requested bucket if authorized.
func (project *Project) GetBucketInfo(bucketName string) (*BucketInfo, error) {
	scope := project.scope.child()

	bucket, _, err := project.lib.GetBucketInfo(scope.ctx, bucketName)
	if err != nil {
		return nil, safeError(err)
	}

	return newBucketInfo(bucket), nil
}

// ListBuckets will list authorized buckets.
func (project *Project) ListBuckets(after string, limit int) (*BucketList, error) {
	scope := project.scope.child()
	opts := libuplink.BucketListOptions{
		Cursor:    after,
		Direction: storj.After,
		Limit:     limit,
	}
	list, err := project.lib.ListBuckets(scope.ctx, &opts)
	if err != nil {
		return nil, safeError(err)
	}

	return &BucketList{list}, nil
}

// DeleteBucket deletes a bucket if authorized. If the bucket contains any
// Objects at the time of deletion, they may be lost permanently.
func (project *Project) DeleteBucket(bucketName string) error {
	scope := project.scope.child()

	err := project.lib.DeleteBucket(scope.ctx, bucketName)
	return safeError(err)
}

func safeError(err error) error {
	// workaround to avoid gomobile panic because of "hash of unhashable type errs.combinedError"
	if err == nil {
		return nil
	}
	return fmt.Errorf("%v", err.Error())
}

// SaltedKeyFromPassphrase returns a key generated from the given passphrase using a stable,
// project-specific salt
func (project *Project) SaltedKeyFromPassphrase(passphrase string) (keyData []byte, err error) {
	scope := project.scope.child()

	key, err := project.lib.SaltedKeyFromPassphrase(scope.ctx, passphrase)
	if err != nil {
		return nil, err
	}
	return key[:], nil
}
