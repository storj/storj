package mobile

import (
	"storj.io/storj/internal/memory"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
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
}

// Uplink represents the main entrypoint to Storj V3. An Uplink connects to
// a specific Satellite and caches connections and resources, allowing one to
// create sessions delineated by specific access controls.
type Uplink struct {
	scope
	lib *libuplink.Uplink
}

// NewUplink creates a new Uplink. This is the first step to create an uplink
// session with a user specified config or with default config, if nil config
func NewUplink(config *Config) (*Uplink, error) {
	scope := rootScope()

	cfg := &libuplink.Config{}
	cfg.Volatile.TLS.SkipPeerCAWhitelist = true
	cfg.Volatile.MaxInlineSize = memory.Size(config.MaxInlineSize)
	cfg.Volatile.MaxMemory = memory.Size(config.MaxMemory)

	lib, err := libuplink.NewUplink(scope.ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Uplink{scope, lib}, nil
}

// Close closes the Uplink. This may not do anything at present, but should
// still be called to allow forward compatibility. No Project or Bucket
// objects using this Uplink should be used after calling Close.
func (uplink *Uplink) Close() error {
	uplink.cancel()
	return uplink.lib.Close()
}

// ProjectOptions allows configuration of various project options during opening
type ProjectOptions struct {
	EncryptionKey []byte
}

// Project represents a specific project access session.
type Project struct {
	scope
	lib *libuplink.Project
}

// OpenProject returns a Project handle with the given APIKey
func (uplink *Uplink) OpenProject(satellite string, apikey string, options *ProjectOptions) (*Project, error) {
	scope := uplink.scope.child()

	opts := libuplink.ProjectOptions{}
	opts.Volatile.EncryptionKey = &storj.Key{}
	copy(opts.Volatile.EncryptionKey[:], options.EncryptionKey) // TODO: error check

	key, err := libuplink.ParseAPIKey(apikey)
	if err != nil {
		return nil, err
	}

	project, err := uplink.lib.OpenProject(scope.ctx, satellite, key, &opts)
	if err != nil {
		return nil, err
	}

	return &Project{scope, project}, nil
}

// Close closes the Project
func (project *Project) Close() error {
	defer project.cancel()
	return project.lib.Close()
}

// CreateBucket creates buckets in project
func (project *Project) CreateBucket(bucketName string, opts *BucketConfig) error {
	scope := project.scope.child()

	cfg := libuplink.BucketConfig{}
	cfg.Volatile.RedundancyScheme = newStorjRedundancyScheme(opts.RedundancyScheme)

	_, err := project.lib.CreateBucket(scope.ctx, bucketName, &cfg)
	if err != nil {
		return err
	}

	return nil
}

// OpenBucket returns a Bucket handle with the given EncryptionAccess
// information.
func (project *Project) OpenBucket(bucketName string, options *BucketAccess) (*Bucket, error) {
	scope := project.scope.child()

	opts := libuplink.EncryptionAccess{}
	copy(opts.Key[:], options.PathEncryptionKey) // TODO: error check
	opts.EncryptedPathPrefix = options.EncryptedPathPrefix

	bucket, err := project.lib.OpenBucket(scope.ctx, bucketName, &opts)
	if err != nil {
		return nil, err
	}

	return &Bucket{bucket.Name, scope, bucket}, nil
}

// GetBucketInfo returns info about the requested bucket if authorized.
func (project *Project) GetBucketInfo(bucketName string) (*BucketInfo, error) {
	scope := project.scope.child()

	bucket, _, err := project.lib.GetBucketInfo(scope.ctx, bucketName)
	if err != nil {
		return nil, err
	}

	return newBucketInfo(bucket), nil
}

// ListBuckets will list authorized buckets.
func (project *Project) ListBuckets(cursor string, direction, limit int) (*BucketList, error) {
	scope := project.scope.child()
	opts := libuplink.BucketListOptions{
		Cursor:    cursor,
		Direction: storj.ListDirection(direction),
		Limit:     limit,
	}
	list, err := project.lib.ListBuckets(scope.ctx, &opts)
	if err != nil {
		return nil, err
	}

	return &BucketList{list}, nil
}

// DeleteBucket deletes a bucket if authorized. If the bucket contains any
// Objects at the time of deletion, they may be lost permanently.
func (project *Project) DeleteBucket(bucketName string) error {
	scope := project.scope.child()

	err := project.lib.DeleteBucket(scope.ctx, bucketName)
	if err != nil {
		return err
	}

	return nil
}
