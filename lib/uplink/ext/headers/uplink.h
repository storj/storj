// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdbool.h>
#include <stdint.h>

typedef __SIZE_TYPE__ GoUintptr;
typedef int64_t Size;

enum ValueType
{
    IDVersionType,
    APIKeyType,
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
typedef struct GoValue APIKey;
typedef GoUintptr IDVersionRef;
typedef struct GoValue IDVersion;
typedef GoUintptr UplinkRef;
typedef struct GoValue Uplink;

struct Config
{
    struct
    {
        struct
        {
            bool SkipPeerCAWhitelist;
            char *PeerCAWhitelistPath;
        } TLS;
        IDVersionRef IdentityVersion;
        char *PeerIDVersion;
        Size MaxInlineSize;
        Size MaxMemory;
    } Volatile;
};
