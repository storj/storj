// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

// #include "uplink_definitions.h"
import "C"

import (
	"storj.io/storj/lib/uplink"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

// Project is a scoped libuplink.Project
type Project struct {
	scope
	lib *libuplink.Project
}

//export CloseProject
// CloseProject closes the project.
func CloseProject(projectHandle C.Project, cerr **C.char) {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return
	}
	universe.Del(projectHandle._handle)
	defer project.cancel()

	if err := project.lib.Close(); err != nil {
		*cerr = C.CString(err.Error())
		return
	}
}

// CreateBucket creates a new bucket if authorized.
//export CreateBucket
func CreateBucket(projectHandle C.Project, name *C.char, bucketConfig *C.BucketConfig, cerr **C.char) C.BucketInfo {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return C.BucketInfo{}
	}

	var config *uplink.BucketConfig
	if bucketConfig != nil {
		config = &uplink.BucketConfig{
			PathCipher: storj.CipherSuite(bucketConfig.path_cipher),
			EncryptionParameters: storj.EncryptionParameters{
				CipherSuite: storj.CipherSuite(bucketConfig.encryption_parameters.cipher_suite),
				BlockSize:   int32(bucketConfig.encryption_parameters.block_size),
			},
		}
		config.Volatile.RedundancyScheme = storj.RedundancyScheme{
			Algorithm:      storj.RedundancyAlgorithm(bucketConfig.redundancy_scheme.algorithm),
			ShareSize:      int32(bucketConfig.redundancy_scheme.share_size),
			RequiredShares: int16(bucketConfig.redundancy_scheme.required_shares),
			RepairShares:   int16(bucketConfig.redundancy_scheme.repair_shares),
			OptimalShares:  int16(bucketConfig.redundancy_scheme.optimal_shares),
			TotalShares:    int16(bucketConfig.redundancy_scheme.total_shares),
		}
	}

	bucket, err := project.lib.CreateBucket(project.scope.ctx, C.GoString(name), config)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.BucketInfo{}
	}

	return newBucketInfo(&bucket)
}

// Bucket is a scoped libuplink.Bucket
type Bucket struct {
	scope
	lib *libuplink.Bucket
}

// OpenBucket returns a Bucket handle with the given EncryptionAccess information.
//export OpenBucket
func OpenBucket(projectHandle C.Project, name *C.char, encryptionAccess C.EncryptionAccess, cerr **C.char) C.Bucket {
	project, ok := universe.Get(projectHandle._handle).(*Project)
	if !ok {
		*cerr = C.CString("invalid project")
		return C.Bucket{}
	}

	var access uplink.EncryptionAccess
	for i := range access.Key {
		access.Key[i] = byte(encryptionAccess.key[0])
	}

	scope := project.scope.child()

	bucket, err := project.lib.OpenBucket(scope.ctx, C.GoString(name), &access)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.Bucket{}
	}

	return C.Bucket{universe.Add(Bucket{scope, bucket})}
}
