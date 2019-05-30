// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdbool.h>
#include <stdint.h>
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

typedef struct EncryptionAccess {
    Bytes_t *key;
} EncryptionAccess_t;

typedef struct UploadOptions {
    char *content_type;
    MapRef_t metadata;
    time_t expires;
} UploadOptions_t;