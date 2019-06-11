#pragma once

#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

typedef struct APIKey   { long _handle; } APIKey;
typedef struct Uplink   { long _handle; } Uplink;
typedef struct Project  { long _handle; } Project;
typedef struct Bucket   { long _handle; } Bucket;
typedef struct Map      { long _handle; } Map;
typedef struct Buffer   { long _handle; } Buffer;
typedef struct Object   { long _handle; } Object;
typedef struct Download { long _handle; } Download;
typedef struct Upload   { long _handle; } Upload;

typedef struct UplinkConfig {
    struct {
        struct {
            uint8_t SkipPeerCAWhitelist;
        } TLS;
    } Volatile;
} UplinkConfig;

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

    int64_t created;
    uint8_t path_cipher;
    int64_t segment_size;

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
    bool      more;
    Bucket_t *items;
    int32_t   length;
} BucketList;

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
} Object;

typedef struct ObjectList {
    char *bucket;
    char *prefix;
    bool more;
    // TODO: use Slice_t{void *items; length int32;?
    Object_t *items;
    int32_t length;
} ObjectList;

typedef struct EncryptionAccess {
    char key[32];
} EncryptionAccess;

typedef struct UploadOptions {
    char *content_type;
    MapRef_t metadata;
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
    char *Bucket;
    char *Path;
    bool IsPrefix;
    char *ContentType;
    MapRef_t MetaData;
    uint64_t Created;
    uint64_t Modified;
    uint64_t Expires;
    uint64_t Size;
    Bytes    Checksum;
} ObjectMeta;