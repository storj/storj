// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdbool.h>
#include <stdint.h>

typedef struct APIKey {
    const char *key;
};

typedef struct TLS {
    bool SkipPeerCAWhitelist;
    char* PeerCAWhitelistPath;
};

typedef struct Volatile {
    struct TLS tls;
    uint8_t IdentityVersion;
    uint32_t* PeerIDVersion;
    uint64_t MaxInlineSize;
    uint64_t MaxMemory;
};

typedef struct Config {
    struct Volatile volatile_;
};
