// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdbool.h>
#include <stdint.h>

typedef __SIZE_TYPE__ GoUintptr;
typedef int64_t Size;

struct APIKey {
    const char *key;
};

struct TLS {
    bool SkipPeerCAWhitelist;
    char* PeerCAWhitelistPath;
};

struct IDVersion {
    uint8_t Number;
    GoUintptr GoIDVersion;
};

struct UplinkConfigVolatile {
    struct TLS TLS;
    struct IDVersion IdentityVersion;
    char* PeerIDVersion;
    Size MaxInlineSize;
    Size MaxMemory;
};

struct Config {
    struct UplinkConfigVolatile Volatile;
};

struct Uplink {
    GoUintptr GoUplink;
    struct Config Config;
};
