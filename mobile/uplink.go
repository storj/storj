package mobile

import (
	"io"

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

func (project *Project) Close() error {
	defer project.cancel()
	return project.lib.Close()
}

type Bucket struct {
	Name string

	scope
	lib *libuplink.Bucket
}

type BucketAccess struct {
	PathEncryptionKey   []byte
	EncryptedPathPrefix storj.Path
}

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

func (bucket *Bucket) Close() error {
	defer bucket.cancel()
	return bucket.lib.Close()
}

type WriterOptions struct {
}

type Writer struct {
	scope
	writer io.WriteCloser
}

func (bucket *Bucket) NewWriter(path storj.Path, options *WriterOptions) (*Writer, error) {
	scope := bucket.scope.child()

	opts := &libuplink.UploadOptions{}
	writer, err := bucket.lib.NewWriter(scope.ctx, path, opts)
	if err != nil {
		return nil, err
	}
	return &Writer{scope, writer}, nil
}

func (w *Writer) Write(data []byte) (int, error) {
	return w.writer.Write(data)
}

func (w *Writer) Close() error {
	defer w.cancel()
	return w.writer.Close()
}

type ReaderOptions struct {
}

type Reader struct {
	scope
	reader interface {
		io.Reader
		io.Seeker
		io.Closer
	}
}

func (bucket *Bucket) NewReader(path storj.Path, options *ReaderOptions) (*Reader, error) {
	scope := bucket.scope.child()

	reader, err := bucket.lib.NewReader(scope.ctx, path)
	if err != nil {
		return nil, err
	}
	return &Reader{scope, reader}, nil
}

func (r *Reader) Read(data []byte) (int, error) {
	return r.reader.Read(data)
}

func (r *Reader) Close() error {
	defer r.cancel()
	return r.reader.Close()
}
