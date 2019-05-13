// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdbool.h>
#include <stdint.h>

typedef __SIZE_TYPE__ GoUintptr;
typedef int64_t Size;
enum ValueType {
    Uplink,
    UplinkConfig
};

typedef GoUintptr APIKey;
typedef GoUintptr IDVersionPtr;
typedef uint8_t IDVersionNumber;
typedef GoUintptr ValuePtr;

struct IDVersion {
    IDVersionNumber Number;
//    IDVersionPtr GoIDVersion;
};

// NB: maybe don't use ValuePtr directly?
struct Value {
    // TODO: use mapping instead
    GoUintptr GoPtr;
    char* Type;
    uint8_t* Snapshot;
    GoUintptr Size;

};

struct Config
{
    struct
    {
        struct
        {
            bool SkipPeerCAWhitelist;
            char *PeerCAWhitelistPath;
        } TLS;
        IDVersionPtr IdentityVersion;
        char *PeerIDVersion;
        Size MaxInlineSize;
        Size MaxMemory;
    } Volatile;
};

struct Uplink
{
    GoUintptr GoUplink;
    struct Config Config;
};
