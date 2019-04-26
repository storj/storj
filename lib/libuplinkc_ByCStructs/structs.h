// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

struct APIKey {
    const char *key;
};

struct TLS {
    bool SkipPeerCAWhitelist
    char* PeerCAWhitelistPath
}

struct Volatile {
    struct TLS tls
    uint8_t IdentityVersion
    uint32_t* PeerIDVersion
    uint64_t MaxInlineSize
    uint64_t MaxMemory
}

struct Config {
    struct Volatile volatile
}