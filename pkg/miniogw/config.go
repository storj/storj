// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package miniogw

// MinioConfig is a configuration struct that keeps details about starting
// Minio
type MinioConfig struct {
	AccessKey string `help:"Minio Access Key to use" default:"insecure-dev-access-key"`
	SecretKey string `help:"Minio Secret Key to use" default:"insecure-dev-secret-key"`
	Dir       string `help:"Minio generic server config path" default:"$CONFDIR/minio"`
}

// ServerConfig determines how minio listens for requests
type ServerConfig struct {
	Address string `help:"address to serve S3 api over" default:"localhost:7777"`
}
