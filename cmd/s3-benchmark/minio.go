// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"io"
	"io/ioutil"

	minio "github.com/minio/minio-go"
	"github.com/zeebo/errs"
)

// MinioError is class for minio errors
var MinioError = errs.Class("minio error")

// Minio implements basic S3 Client with minio
type Minio struct {
	api *minio.Client
}

// NewMinio creates new Client
func NewMinio(conf Config) (Client, error) {
	api, err := minio.New(conf.Endpoint, conf.AccessKey, conf.SecretKey, !conf.NoSSL)
	if err != nil {
		return nil, MinioError.Wrap(err)
	}
	return &Minio{api}, nil
}

// MakeBucket makes a new bucket
func (client *Minio) MakeBucket(bucket, location string) error {
	err := client.api.MakeBucket(bucket, location)
	if err != nil {
		return MinioError.Wrap(err)
	}
	return nil
}

// RemoveBucket removes a bucket
func (client *Minio) RemoveBucket(bucket string) error {
	err := client.api.RemoveBucket(bucket)
	if err != nil {
		return MinioError.Wrap(err)
	}
	return nil
}

// ListBuckets lists all buckets
func (client *Minio) ListBuckets() ([]string, error) {
	buckets, err := client.api.ListBuckets()
	if err != nil {
		return nil, MinioError.Wrap(err)
	}

	names := []string{}
	for _, bucket := range buckets {
		names = append(names, bucket.Name)
	}
	return names, nil
}

// Upload uploads object data to the specified path
func (client *Minio) Upload(bucket, objectName string, data []byte) error {
	_, err := client.api.PutObject(
		bucket, objectName,
		bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return MinioError.Wrap(err)
	}
	return nil
}

// UploadMultipart uses multipart uploads, has hardcoded threshold
func (client *Minio) UploadMultipart(bucket, objectName string, data []byte, threshold int) error {
	_, err := client.api.PutObject(
		bucket, objectName,
		bytes.NewReader(data), -1,
		minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return MinioError.Wrap(err)
	}
	return nil
}

// Download downloads object data
func (client *Minio) Download(bucket, objectName string, buffer []byte) ([]byte, error) {
	reader, err := client.api.GetObject(bucket, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, MinioError.Wrap(err)
	}
	defer func() { _ = reader.Close() }()

	n, err := reader.Read(buffer[:cap(buffer)])
	if err != io.EOF {
		rest, err := ioutil.ReadAll(reader)
		if err == io.EOF {
			err = nil
		}
		if err != nil {
			return nil, MinioError.Wrap(err)
		}
		buffer = append(buffer, rest...)
		n = len(buffer)
	}

	buffer = buffer[:n]
	return buffer, nil
}

// Delete deletes object
func (client *Minio) Delete(bucket, objectName string) error {
	err := client.api.RemoveObject(bucket, objectName)
	if err != nil {
		return MinioError.Wrap(err)
	}
	return nil
}

// ListObjects lists objects
func (client *Minio) ListObjects(bucket, prefix string) ([]string, error) {
	doneCh := make(chan struct{})
	defer close(doneCh)

	names := []string{}
	for message := range client.api.ListObjects(bucket, prefix, false, doneCh) {
		names = append(names, message.Key)
	}

	return names, nil
}
