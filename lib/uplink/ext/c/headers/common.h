// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdint.h>

typedef __SIZE_TYPE__ GoUintptr;
typedef int64_t Size;

enum ValueType
{
    IDVersionType,
    APIKeyType,
    UplinkConfigType,
    UplinkType,
    ProjectType,
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
typedef struct GoValue gvAPIKey;
typedef GoUintptr IDVersionRef;
typedef struct GoValue gvIDVersion;

void *UnpackValue(struct GoValue *, char **);