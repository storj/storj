// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"
	"crypto"
	"crypto/x509"
	"io"
	"time"

	"storj.io/storj/pkg/miniogw"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storj"
)

// An Identity is a parsed leaf cert keypair with a certificate signed chain
// up to the self-signed CA with the node ID.
type Identity interface {
	NodeID() storj.NodeID
	Key() crypto.PrivateKey
	Certs() []x509.Certificate
}

// Caveats could be things like a read-only restriction, a time-bound
// restriction, a bucket-specific restriction, a path-prefix restriction, a
// full path restriction, etc.
type Caveat interface {
}

// A Macaroon represents an access credential to certain resources
type Macaroon interface {
	Serialize() ([]byte, error)
	Restrict(caveats ...Caveat) Macaroon
}

type Config struct {
	// MaxBufferMem controls upload performance and is system-specific
	MaxBufferMem int

	// These should only be relevant for new files; these values for existing
	// files should come from the metainfo index. It's unlikely these will ever
	// change much.
	EncBlockSize  int
	MaxInlineSize int
	SegmentSize   int64
}

// Uplink represents the main entrypoint to Storj V3. An Uplink connects to
// a specific Satellite and caches connections and resources, allowing one to
// create sessions delineated by specific access controls.
type Uplink struct {
}

// NewUplink creates a new Uplink
func NewUplink(identity Identity, satelliteAddr string, cfg Config) *Uplink {
	panic("TODO")
}

// A Share is all of the access information an application needs to store and
// retrieve data. Someone with a share may have no restrictions within a project
// (can create buckets, list buckets, list files, upload files, delete files,
// etc), may be restricted to a single bucket, may be restricted to a prefix
// within a bucket, or may even be restricted to a single file within a bucket.
type Share struct {
	Access Macaroon

	// TODO: these should be per-bucket somehow maybe? oh man what a nightmare
	PathCipher       storj.Cipher
	EncPathPrefix    storj.Path
	Key              storj.Key
	EncryptionScheme storj.EncryptionScheme
}

// ParseShare parses a serialized Share
func ParseShare(data []byte) (Share, error) {
	panic("TODO")
}

func (s *Share) Serialize() ([]byte, error) {
	panic("TODO")
}

// Session represents a specific access session.
type Session struct {
}

// A Session is created with a Share.
func (u *Uplink) Session(share Share) *Session {
	panic("TODO")
}

// GetBucket returns info about the requested bucket if authorized
func (s *Session) GetBucket(ctx context.Context, bucket string) (storj.Bucket,
	error) {
	panic("TODO")
}

type CreateBucketOptions struct {
	PathCipher Cipher
	// this differs from storj.CreateBucket's choice of just using storj.Bucket
	// by not having 2/3 unsettable fields.
}

// CreateBucket creates a new bucket if authorized
func (s *Session) CreateBucket(ctx context.Context, bucket string,
	opts *CreateBucketOptions) (storj.Bucket, error) {
	panic("TODO")
}

// DeleteBucket deletes a bucket if authorized
func (s *Session) DeleteBucket(ctx context.Context, bucket string) error {
	panic("TODO")
}

// ListBuckets will list authorized buckets
func (s *Session) ListBuckets(ctx context.Context, opts storj.BucketListOptions) (
	storj.BucketList, error) {
	panic("TODO")
}

// Share creates a new share, potentially further restricted from the Share used
// to create this session.
func (s *Session) Share(ctx context.Context, caveats ...Caveat) (Share, error) {
	panic("TODO")
}

// ObjectMeta represents metadata about a specific Object
type ObjectMeta struct {
	Bucket   string
	Path     storj.Path
	IsPrefix bool

	Metadata map[string]string

	Created  time.Time
	Modified time.Time
	Expires  time.Time

	Size     int64
	Checksum string

	// this differs from storj.Object by not having Version (yet), and not
	// having a Stream embedded. I'm also not sold on splitting ContentType out
	// from Metadata but shrugemoji.
}

// GetObject returns a handle to the data for an object and its metadata, if
// authorized.
func (s *Session) GetObject(ctx context.Context, bucket string, path storj.Path) (
	ranger.Ranger, ObjectMeta, error) {
	panic("TODO")
}

// ObjectPutOpts controls options about uploading a new Object, if authorized.
type ObjectPutOpts struct {
	Metadata map[string]string
	Expires  time.Time

	// the satellite should probably tell the uplink what to use for these
	// per bucket. also these should probably be denormalized and defined here.
	RS            *storj.RedundancyScheme
	NodeSelection *miniogw.NodeSelectionConfig
}

// PutObject uploads a new object, if authorized.
func (s *Session) PutObject(ctx context.Context, bucket string, path storj.Path,
	data io.Reader, opts ObjectPutOpts) error {
	panic("TODO")
}

// DeleteObject removes an object, if authorized.
func (s *Session) DeleteObject(ctx context.Context, bucket string,
	path storj.Path) error {
	panic("TODO")
}

type ListObjectsField int

const (
	ListObjectsMetaNone        ListObjectsField = 0
	ListObjectsMetaModified    ListObjectsField = 1 << iota
	ListObjectsMetaExpiration  ListObjectsField = 1 << iota
	ListObjectsMetaSize        ListObjectsField = 1 << iota
	ListObjectsMetaChecksum    ListObjectsField = 1 << iota
	ListObjectsMetaUserDefined ListObjectsField = 1 << iota
	ListObjectsMetaAll         ListObjectsField = 1 << iota
)

type ListObjectsConfig struct {
	// this differs from storj.ListOptions by removing the Delimiter field
	// (ours is hardcoded as '/'), and adding the Fields field to optionally
	// support efficient listing that doesn't require looking outside of the
	// path index in pointerdb.

	Prefix    storj.Path
	Cursor    storj.Path
	Recursive bool
	Direction storj.ListDirection
	Limit     int
	Fields    ListObjectsFields
}

// ListObjects lists objects a user is authorized to see.
func (s *Session) ListObjects(ctx context.Context, bucket string,
	cfg ListObjectsConfig) (items []ObjectMeta, more bool, err error) {
	panic("TODO")
}

// NewPartialUpload starts a new partial upload and returns that partial
// upload id
func (s *Session) NewPartialUpload(ctx context.Context, bucket string) (
	uploadID string, err error) {
	panic("TODO")
}

// TODO: lists upload ids
func (s *Session) ListPartialUploads() {
	panic("TODO")
}

// TODO: adds a new segment with given RS and node selection config
func (s *Session) PutPartialUpload() {
	panic("TODO")
}

// TODO: takes a path, metadata, etc, and puts all of the segment metadata
// into place. the object doesn't show up until this method is called.
func (s *Session) FinishPartialUpload() {
	panic("TODO")
}

// AbortPartialUpload cancels an existing partial upload.
func (s *Session) AbortPartialUpload(ctx context.Context,
	bucket, uploadID string) error {
	panic("TODO")
}
