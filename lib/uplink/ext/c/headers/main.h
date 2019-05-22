// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdbool.h>
#include <stdint.h>
#include "../pb/uplink.pb-c.h"

typedef __SIZE_TYPE__ GoUintptr;
typedef int64_t Size;

enum ValueType
{
    IDVersionType,
    APIKeyType,
    UplinkConfigType,
    UplinkType,
    ProjectType,
    ProjectOptionsType,
    BucketType
};

struct GoValue
{
    GoUintptr Ptr;
    enum ValueType Type;
    uint8_t *Snapshot;
    GoUintptr Size;
};

typedef GoUintptr APIKeyRef;
typedef GoUintptr IDVersionRef;
typedef GoUintptr UplinkRef;
typedef GoUintptr UplinkConfigRef;
typedef GoUintptr ProjectRef;
typedef GoUintptr BucketRef;


// GoValue type aliases
typedef struct GoValue gvAPIKey;
typedef struct GoValue gvIDVersion;
typedef struct GoValue gvUplink;
typedef struct GoValue gvUplinkConfig;
typedef struct GoValue gvProjectOptions;
typedef struct GoValue gvBucket;
typedef struct GoValue gvBucketConfig;

// Protobuf aliases
typedef Storj__Libuplink__IDVersion pbIDVersion;
typedef Storj__Libuplink__UplinkConfig pbUplinkConfig;
typedef Storj__Libuplink__TLSConfig pbTLSConfig;
typedef Storj__Libuplink__ProjectOptions pbProjectOptions;
typedef Storj__Libuplink__BucketConfig pbBucketConfig;
typedef Storj__Libuplink__RedundancyScheme pbRedundancyScheme;
typedef Storj__Libuplink__EncryptionParameters pbEncryptionParameters;

void *get_snapshot(struct GoValue *, char **);
void protoToGoValue(void *, struct GoValue *, char **);
