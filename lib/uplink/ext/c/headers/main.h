// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <time.h>

typedef __SIZE_TYPE__ GoUintptr;
typedef int64_t Size;

typedef GoUintptr APIKeyRef_t;
typedef GoUintptr IDVersionRef_t;
typedef GoUintptr UplinkRef_t;
typedef GoUintptr UplinkConfigRef_t;
typedef GoUintptr ProjectRef_t;
typedef GoUintptr BucketRef_t;
typedef GoUintptr BucketConfigRef_t;
typedef GoUintptr MapRef_t;
typedef GoUintptr BufferRef_t;
typedef GoUintptr ObjectRef_t;
typedef GoUintptr DownloadReaderRef_t;
typedef GoUintptr UploadReaderRef_t;

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
    EncryptionParameters_t *encryption_parameters;
    RedundancyScheme_t *redundancy_scheme;
    char *name;
    int64_t created;
    uint8_t path_cipher;
    int64_t segment_size;
} Bucket_t;

typedef struct BucketConfig {
    EncryptionParameters_t *encryption_parameters;
    RedundancyScheme_t *redundancy_scheme;
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

typedef struct ListOptions {
    char *prefix;
    char *cursor;
    char delimiter;
    bool recursive;
    int8_t direction;
    int64_t limit;
} ListOptions_t;

typedef struct ObjectMeta {
	char *Bucket;
	char *Path;
	bool IsPrefix;
	char *ContentType;
	MapRef_t Metadata;
	uint64_t Created;
	uint64_t Modified;
	uint64_t Expires;
	uint64_t Size;
	Bytes_t Checksum;
} ObjectMeta_t;