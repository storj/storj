// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

/*
#include <stdint.h>
#include <stdbool.h>

typedef long long APIKeyRef_t;
typedef long long IDVersionRef_t;
typedef long long UplinkRef_t;
typedef long long UplinkConfigRef_t;
typedef long long ProjectRef_t;
typedef long long BucketRef_t;
typedef long long BucketConfigRef_t;
typedef long long MapRef_t;
typedef long long BufferRef_t;
typedef long long ObjectRef_t;
typedef long long DownloadReaderRef_t;
typedef long long UploadReaderRef_t;

// TODO: Add free functions for each struct

typedef struct Bytes {
	uint8_t *bytes;
	int32_t length;
} Bytes_t;

typedef struct IDVersion {
	uint16_t number;
} IDVersion_t;

typedef struct EncryptionParameters {
	uint8_t cipher_suite;
	int32_t block_size;
} EncryptionParameters_t;

typedef struct RedundancyScheme {
	uint8_t algorithm;
	int32_t share_size;
	int16_t required_shares;
	int16_t repair_shares;
	int16_t optimal_shares;
	int16_t total_shares;
} RedundancyScheme_t;

typedef struct Bucket {
	EncryptionParameters_t encryption_parameters;
	RedundancyScheme_t redundancy_scheme;
	char *name;
	int64_t created;
	uint8_t path_cipher;
	int64_t segment_size;
} Bucket_t;

typedef struct BucketConfig {
	EncryptionParameters_t encryption_parameters;
	RedundancyScheme_t redundancy_scheme;
	uint8_t path_cipher;
} BucketConfig_t;

typedef struct BucketInfo {
	Bucket_t bucket;
	BucketConfig_t config;
} BucketInfo_t;

typedef struct BucketListOptions {
	char *cursor;
	int8_t direction;
	int64_t limit;
} BucketListOptions_t;

typedef struct BucketList {
	bool more;
	Bucket_t *items;
	int32_t length;
} BucketList_t;

typedef struct Object {
	uint32_t version;
	Bucket_t bucket;
	char *path;
	bool is_prefix;
	MapRef_t metadata;
	char *content_type;
	time_t created;
	time_t modified;
	time_t expires;
} Object_t;

typedef struct ObjectList {
	char *bucket;
	char *prefix;
	bool more;
	// TODO: use Slice_t{void *items; length int32_t;?
	Object_t *items;
	int32_t length;
} ObjectList_t;

typedef struct EncryptionAccess {
	Bytes_t *key;
} EncryptionAccess_t;

typedef struct UploadOptions {
	char *content_type;
	MapRef_t metadata;
	time_t expires;
} UploadOptions_t;

typedef struct ObjectListOptions {
	char *prefix;
	char *cursor;
	char delimiter;
	bool recursive;
	int8_t direction;
	int64_t limit;
} ObjectListOptions_t;

typedef struct ObjectMeta {
	char *Bucket;
	char *Path;
	bool IsPrefix;
	char *ContentType;
	MapRef_t MetaData;
	uint64_t Created;
	uint64_t Modified;
	uint64_t Expires;
	uint64_t Size;
	Bytes_t Checksum;
} ObjectMeta_t;
*/
import "C"

import (
	"errors"

	"gopkg.in/spacemonkeygo/monkit.v2"

	libuplink "storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/storj"
)

var mon = monkit.Package()

//export GetIDVersion
func GetIDVersion(number C.uint8_t, cerr **C.char) C.IDVersion_t {
	version, err := storj.GetIDVersion(storj.IDVersionNumber(number))
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.IDVersion_t{}
	}

	return C.IDVersion_t{
		number: C.uint16_t(version.Number),
	}
}

//export ParseAPIKey
func ParseAPIKey(val *C.char, cerr **C.char) C.APIKeyRef_t {
	apikey, err := libuplink.ParseAPIKey(C.GoString(val))
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.APIKeyRef_t(0)
	}

	return C.APIKeyRef_t(universe.Add(apikey))
}

//export Serialize
func Serialize(cAPIKey C.APIKeyRef_t) *C.char {
	apikey, ok := universe.Get(Ref(cAPIKey)).(libuplink.APIKey)
	if !ok {
		return C.CString("")
	}

	return C.CString(apikey.Serialize())
}

type Uplink struct {
	scope
	lib *libuplink.Uplink
}

//export NewUplink
func NewUplink(cerr **C.char) C.UplinkRef_t {
	scope := rootScope("inmemory")

	cfg := &libuplink.Config{}
	lib, err := libuplink.NewUplink(scope.ctx, cfg)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.UplinkRef_t(0)
	}

	return C.UplinkRef_t(universe.Add(&Uplink{scope, lib}))
}

//export NewUplinkInsecure
func NewUplinkInsecure(cerr **C.char) C.UplinkRef_t {
	scope := rootScope("inmemory")

	cfg := &libuplink.Config{}
	cfg.Volatile.TLS.SkipPeerCAWhitelist = true
	lib, err := libuplink.NewUplink(scope.ctx, cfg)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.UplinkRef_t(0)
	}

	return C.UplinkRef_t(universe.Add(&Uplink{scope, lib}))
}

type Project struct {
	scope
	lib *libuplink.Project
}

//export OpenProject
func OpenProject(uplinkref C.UplinkRef_t, satelliteAddr *C.char, cAPIKey C.APIKeyRef_t, cerr **C.char) C.ProjectRef_t {
	uplink, ok := universe.Get(Ref(uplinkref)).(*Uplink)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return C.ProjectRef_t(0)
	}

	var err error
	defer mon.Task()(&uplink.scope.ctx)(&err)

	apikey, ok := universe.Get(Ref(cAPIKey)).(libuplink.APIKey)
	if !ok {
		err = errors.New("invalid API Key")
		*cerr = C.CString(err.Error())
		return C.ProjectRef_t(0)
	}

	scope := uplink.scope.child()

	// TODO: add project options argument
	var project *libuplink.Project
	project, err = uplink.lib.OpenProject(scope.ctx, C.GoString(satelliteAddr), apikey, nil)
	if err != nil {
		*cerr = C.CString(err.Error())
		return C.ProjectRef_t(0)
	}

	return C.ProjectRef_t(universe.Add(&Project{scope, project}))
}

//export CloseProject
func CloseProject(projectref C.ProjectRef_t, cerr **C.char) {
	project, ok := universe.Get(Ref(projectref)).(*Project)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return
	}
	universe.Del(Ref(projectref))
	defer project.cancel()

	if err := project.lib.Close(); err != nil {
		*cerr = C.CString(err.Error())
		return
	}
}

//export CloseUplink
func CloseUplink(uplinkref C.UplinkRef_t, cerr **C.char) {
	uplink, ok := universe.Get(Ref(uplinkref)).(*Uplink)
	if !ok {
		*cerr = C.CString("invalid uplink")
		return
	}
	universe.Del(Ref(uplinkref))
	defer uplink.cancel()

	if err := uplink.lib.Close(); err != nil {
		*cerr = C.CString(err.Error())
		return
	}
}
