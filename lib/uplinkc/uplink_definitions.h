#pragma once

#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>

typedef struct APIKey     { long _handle; } APIKeyRef;
typedef struct Uplink     { long _handle; } UplinkRef;
typedef struct Project    { long _handle; } ProjectRef;
typedef struct Bucket     { long _handle; } BucketRef;
typedef struct Metadata   { long _handle; } MetadataRef;
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
    char                 *name;
    int64_t              created;
    uint8_t              path_cipher;
    uint64_t             segment_size;
    EncryptionParameters encryption_parameters;
    RedundancyScheme     redundancy_scheme;
} BucketInfo;

typedef struct BucketConfig {
    uint8_t              path_cipher;
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

typedef struct UploadOptions {
    char    *content_type;
    MetadataRef  metadata;
    int64_t expires;
} UploadOptions;
