// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdbool.h>
#include <stdint.h>

typedef __SIZE_TYPE__ GoUintptr;
typedef int64_t Size;

typedef GoUintptr APIKeyRef_t;
typedef GoUintptr IDVersionRef_t;
typedef GoUintptr UplinkRef_t;
typedef GoUintptr UplinkConfigRef_t;
typedef GoUintptr ProjectRef_t;
typedef GoUintptr BucketRef_t;
typedef GoUintptr BucketConfigRef_t;

struct IDVersion {
    uint16_t number;
};
typedef struct IDVersion IDVersion_t;

struct EncryptionParameters {
    uint8_t cipher_suite;
    int32_t block_size;
};
typedef struct EncryptionParameters EncryptionParameters_t;

struct BucketConfig {
    EncryptionParameters_t *encryption_parameters;
    uint8_t path_cipher;
};
typedef struct BucketConfig BucketConfig_t;

struct RedundancyScheme {
    uint8_t algorithm;
    int32_t share_size;
    int16_t required_shares;
    int16_t repair_shares;
    int16_t optimal_shares;
    int16_t total_shares;
};
typedef struct RedundancyScheme RedundancyScheme_t;

struct Bucket {
    EncryptionParameters_t *encryption_parameters;
    RedundancyScheme_t *redundancy_scheme;
    char *name;
    int64_t created;
    uint8_t path_cipher;
    int64_t segment_size;
};
typedef struct Bucket Bucket_t;
