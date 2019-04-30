// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdbool.h>
#include <stdint.h>

typedef __SIZE_TYPE__ GoUintptr;

struct APIKey {
    const char *key;
};

struct TLS {
    bool SkipPeerCAWhitelist;
    char* PeerCAWhitelistPath;
};

struct Volatile {
    struct TLS tls;
    uint8_t IdentityVersion;
    uint32_t* PeerIDVersion;
    uint64_t MaxInlineSize;
    uint64_t MaxMemory;
};

struct Config {
    struct Volatile Volatile;
};

struct Uplink {
    GoUintptr GoUplink;
    struct Config Config;
};
