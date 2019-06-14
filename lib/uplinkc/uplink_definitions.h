#pragma once

#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

typedef struct APIKey   { long _handle; } APIKeyRef_t;
typedef struct Uplink   { long _handle; } UplinkRef_t;
typedef struct Project  { long _handle; } ProjectRef_t;
typedef struct Bucket   { long _handle; } BucketRef_t;

typedef int64_t time_t;

typedef struct UplinkConfig {
    struct {
        struct {
            bool SkipPeerCAWhitelist;
        } TLS;
    } Volatile;
} UplinkConfig_t;

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

typedef struct BucketInfo {
    char    *name;

    int64_t created;
    uint8_t path_cipher;
    uint64_t segment_size;

    EncryptionParameters_t encryption_parameters;
    RedundancyScheme_t     redundancy_scheme;
} BucketInfo_t;

typedef struct BucketConfig {
    uint8_t path_cipher;

    EncryptionParameters_t encryption_parameters;
    RedundancyScheme_t     redundancy_scheme;
} BucketConfig_t;

typedef struct BucketListOptions {
    char    *cursor;
    int8_t  direction;
    int64_t limit;
} BucketListOptions_t;

typedef struct BucketList {
    bool       more;
    BucketInfo_t *items;
    int32_t    length;
} BucketList_t;

typedef struct EncryptionAccess {
    char key[32];
} EncryptionAccess_t;
