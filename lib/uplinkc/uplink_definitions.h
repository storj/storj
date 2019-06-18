#pragma once

#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <time.h>

typedef struct APIKey     { long _handle; } APIKeyRef;
typedef struct Uplink     { long _handle; } UplinkRef;
typedef struct Project    { long _handle; } ProjectRef;
typedef struct Bucket     { long _handle; } BucketRef;
typedef struct Map        { long _handle; } MapRef;
typedef struct Object     { long _handle; } ObjectRef;
typedef struct Downloader { long _handle; } DownloaderRef;
typedef struct Uploader   { long _handle; } UploaderRef;

typedef struct UplinkConfig {
    struct {
        struct {
            bool SkipPeerCAWhitelist;
        } TLS;
    } Volatile;
} UplinkConfig;

typedef struct ProjectOptions {
    char key[32];
} ProjectOptions;

typedef struct EncryptionParameters {
    uint8_t cipher_suite;
    int32_t block_size;
} EncryptionParameters;

typedef struct RedundancyScheme {
    uint8_t algorithm;
    int32_t share_size;
    int16_t required_shares;
    int16_t repair_shares;
    int16_t optimal_shares;
    int16_t total_shares;
} RedundancyScheme;

typedef struct BucketInfo {
    char    *name;

    time_t created;
    uint8_t path_cipher;
    uint64_t segment_size;

    EncryptionParameters encryption_parameters;
    RedundancyScheme     redundancy_scheme;
} BucketInfo;

typedef struct BucketConfig {
    uint8_t path_cipher;

    EncryptionParameters encryption_parameters;
    RedundancyScheme     redundancy_scheme;
} BucketConfig;

typedef struct BucketListOptions {
    char    *cursor;
    int8_t  direction;
    int64_t limit;
} BucketListOptions;

typedef struct BucketList {
    bool       more;
    BucketInfo *items;
    int32_t    length;
} BucketList;

typedef struct EncryptionAccess {
    char key[32];
} EncryptionAccess;

typedef struct ObjectInfo {
    uint32_t   version;
    BucketInfo bucket;
    char       *path;
    bool       is_prefix;
    MapRef        metadata;
    char       *content_type;
    time_t     created;
    time_t     modified;
    time_t     expires;
} ObjectInfo;

typedef struct ObjectList {
    char *bucket;
    char *prefix;
    bool more;
    ObjectInfo *items;
    int32_t length;
} ObjectList;

typedef struct UploadOptions {
    char *content_type;
    MapRef    metadata;
    time_t expires;
} UploadOptions;

typedef struct ListOptions {
    char *prefix;
    char *cursor;
    char delimiter;
    bool recursive;
    int8_t direction;
    int64_t limit;
} ListOptions;

typedef struct ObjectMeta {
    char *bucket;
    char *path;
    bool is_prefix;
    char *content_type;
    MapRef meta_data;
    time_t created;
    time_t modified;
    time_t expires;
    uint64_t size;
    uint8_t *checksum_bytes;
    uint64_t checksum_length;
} ObjectMeta;