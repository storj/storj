// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"context"

	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
)

// APIKey represents an access credential to certain resources
type APIKey = uplink.APIKey

// ParseAPIKey parses an API key
func ParseAPIKey(val string) (APIKey, error) {
	return uplink.ParseAPIKey(val)
}

// Bucket represents operations you can perform on a bucket
type Bucket = uplink.Bucket

// UploadOptions controls options about uploading a new Object, if authorized.
type UploadOptions = uplink.UploadOptions

// ListOptions controls options for the ListObjects() call.
type ListOptions = uplink.ListOptions

// EncryptionAccess represents an encryption access context. It holds
// information about how various buckets and objects should be
// encrypted and decrypted.
type EncryptionAccess = uplink.EncryptionAccess

// NewEncryptionAccess creates an encryption access context
func NewEncryptionAccess() *EncryptionAccess {
	return uplink.NewEncryptionAccess()
}

// NewEncryptionAccessWithDefaultKey creates an encryption access context with
// a default key set.
// Use (*Project).SaltedKeyFromPassphrase to generate a default key
func NewEncryptionAccessWithDefaultKey(defaultKey storj.Key) *EncryptionAccess {
	return uplink.NewEncryptionAccessWithDefaultKey(defaultKey)
}

// EncryptionRestriction represents a scenario where some set of objects
// may need to be encrypted/decrypted
type EncryptionRestriction = uplink.EncryptionRestriction

// ParseEncryptionAccess parses a base58 serialized encryption access into a working one.
func ParseEncryptionAccess(serialized string) (*EncryptionAccess, error) {
	return uplink.ParseEncryptionAccess(serialized)
}

// ObjectMeta contains metadata about a specific Object.
type ObjectMeta = uplink.ObjectMeta

// An Object is a sequence of bytes with associated metadata, stored in the
// Storj network (or being prepared for such storage). It belongs to a specific
// bucket, and has a path and a size. It is comparable to a "file" in a
// conventional filesystem.
type Object = uplink.Object

// Project represents a specific project access session.
type Project = uplink.Project

// BucketConfig holds information about a bucket's configuration. This is
// filled in by the caller for use with CreateBucket(), or filled in by the
// library as Bucket.Config when a bucket is returned from OpenBucket().
type BucketConfig = uplink.BucketConfig

// BucketListOptions controls options to the ListBuckets() call.
type BucketListOptions = uplink.BucketListOptions

// Scope is a serializable type that represents all of the credentials you need
// to open a project and some amount of buckets
type Scope = uplink.Scope

// ParseScope unmarshals a base58 encoded scope protobuf and decodes
// the fields into the Scope convenience type. It will return an error if the
// protobuf is malformed or field validation fails.
func ParseScope(scopeb58 string) (*Scope, error) {
	return uplink.ParseScope(scopeb58)
}

// Config represents configuration options for an Uplink
type Config = uplink.Config

// Uplink represents the main entrypoint to Storj V3. An Uplink connects to
// a specific Satellite and caches connections and resources, allowing one to
// create sessions delineated by specific access controls.
type Uplink = uplink.Uplink

// NewUplink creates a new Uplink. This is the first step to create an uplink
// session with a user specified config or with default config, if nil config
func NewUplink(ctx context.Context, cfg *Config) (_ *Uplink, err error) {
	return uplink.NewUplink(ctx, cfg)
}
