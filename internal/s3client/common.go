// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package s3client

// Config is the setup for a particular client
type Config struct {
	S3Gateway     string
	Satellite     string
	AccessKey     string
	SecretKey     string
	APIKey        string
	EncryptionKey string
	NoSSL         bool
}

// Client is the common interface for different implementations
type Client interface {
	MakeBucket(bucket, location string) error
	RemoveBucket(bucket string) error
	ListBuckets() ([]string, error)

	Upload(bucket, objectName string, data []byte) error
	Download(bucket, objectName string, buffer []byte) ([]byte, error)
	Delete(bucket, objectName string) error
	ListObjects(bucket, prefix string) ([]string, error)
}
