// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdbool.h>
#include <stdint.h>

typedef __SIZE_TYPE__ GoUintptr;
typedef int64_t Size;
enum ValueType { IDVersionType };

typedef GoUintptr APIKey;
typedef uint8_t IDVersionNumber;

struct IDVersion {
    IDVersionNumber Number;
//    IDVersionPtr GoIDVersion;
};

// NB: maybe don't use ValuePtr directly?
struct GoValue {
    // TODO: use mapping instead
    GoUintptr Ptr;
    enum ValueType Type;
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
        struct IDVersion IdentityVersion;
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
