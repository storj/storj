package mobile

import (
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

type Config struct {
	Identity []byte
}

type Uplink struct {
	scope
	lib *libuplink.Uplink
}

func NewUplink(config *Config) (*Uplink, error) {
	scope := rootScope()

	cfg := &libuplink.Config{}
	cfg.Volatile.UseIdentity = nil // TODO
	cfg.Volatile.TLS.SkipPeerCAWhitelist = true
	lib, err := libuplink.NewUplink(scope.ctx, cfg)
	if err != nil {
		return nil, err
	}
	return &Uplink{scope, lib}, nil
}

func (uplink *Uplink) Close() error {
	uplink.cancel()
	return uplink.lib.Close()
}

type ProjectOptions struct {
	EncryptionKey []byte
}

type Project struct {
	scope
	lib *libuplink.Project
}

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

// Close project
func (project *Project) Close() error {
	defer project.cancel()
	return project.lib.Close()
}

// CreateBucket creates buckets in project
func (project *Project) CreateBucket(bucketName string) error {
	scope := project.scope.child()

	opts := libuplink.BucketConfig{}
	_, err := project.lib.CreateBucket(scope.ctx, bucketName, &opts)
	if err != nil {
		return err
	}

	return nil
}

// OpenBucket test
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

// GetBucketInfo gets bucket info
func (project *Project) GetBucketInfo(bucketName string) (*BucketInfo, error) {
	scope := project.scope.child()

	bucket, _, err := project.lib.GetBucketInfo(scope.ctx, bucketName)
	if err != nil {
		return nil, err
	}

	return newBucketInfo(bucket), nil
}

// ListBuckets lists buckets in project
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

// DeleteBucket deletes bucket from project
func (project *Project) DeleteBucket(bucketName string) error {
	scope := project.scope.child()

	err := project.lib.DeleteBucket(scope.ctx, bucketName)
	if err != nil {
		return err
	}

	return nil
}
