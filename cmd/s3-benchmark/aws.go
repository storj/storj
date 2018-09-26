package main

import (
	"bytes"
	"os/exec"

	"github.com/zeebo/errs"
)

// AWSError is class for minio errors
var AWSError = errs.Class("aws error")

// AWS implements basic S3 Client with aws-cli
type AWS struct {
	conf Config
}

// NewAWS creates new Client
func NewAWS(conf Config) (Client, error) {
	return &AWS{conf}, nil
}

func (client *AWS) cmd(args ...string) *exec.Cmd {
	// TODO: add arguments from config
	return exec.Command("aws", args...)
}

// MakeBucket makes a new bucket
func (client *AWS) MakeBucket(bucket, location string) error {
	cmd := client.cmd("s3", "mb", "s3://"+bucket, "--region", location)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return AWSError.Wrap(err)
	}
	return nil
}

// RemoveBucket removes a bucket
func (client *AWS) RemoveBucket(bucket string) error {
	cmd := client.cmd("s3", "rb", "s3://"+bucket)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return AWSError.Wrap(err)
	}
	return nil
}

// ListBuckets lists all buckets
func (client *AWS) ListBuckets() ([]string, error) {
	cmd := client.cmd("s3api", "list-buckets", "--output", "json")
	buckets, err := cmd.CombinedOutput()
	if err != nil {
		return nil, AWSError.Wrap(err)
	}
	// TODO parse json
	return []string{string(buckets)}, nil
}

// Upload uploads object data to the specified path
func (client *AWS) Upload(bucket, objectName string, data []byte) error {
	// TODO: add upload threshold
	cmd := client.cmd("s3", "cp", "-", "s3://"+bucket+"/"+objectName)
	cmd.Stdin = bytes.NewReader(data)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return AWSError.Wrap(err)
	}
	return nil
}

// UploadMultipart uses multipart uploads, has hardcoded threshold
func (client *AWS) UploadMultipart(bucket, objectName string, data []byte, threshold int) error {
	// TODO: add upload threshold
	cmd := client.cmd("s3", "cp", "-", "s3://"+bucket+"/"+objectName)
	cmd.Stdin = bytes.NewReader(data)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return AWSError.Wrap(err)
	}
	return nil
}

// Download downloads object data
func (client *AWS) Download(bucket, objectName string, buffer []byte) ([]byte, error) {
	cmd := client.cmd("s3", "cp", "s3://"+bucket+"/"+objectName, "-")

	buf := bytes.NewBuffer(buffer)
	cmd.Stdout = buf

	err := cmd.Run()
	if err != nil {
		return nil, AWSError.Wrap(err)
	}

	return buf.Bytes(), nil
}

// Delete deletes object
func (client *AWS) Delete(bucket, objectName string) error {
	cmd := client.cmd("s3", "rb", "s3://"+bucket+"/"+objectName)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return AWSError.Wrap(err)
	}
	return nil
}

// ListObject list objects
func (client *AWS) ListObjects(bucket, prefix string) ([]string, error) {
	cmd := client.cmd("s3api", "list-objects",
		"--output", "json",
		"--bucket", bucket,
		"--prefix", prefix)
	data, err := cmd.CombinedOutput()
	if err != nil {
		return nil, AWSError.Wrap(err)
	}
	// TODO parse json
	return []string{string(data)}, nil
}
