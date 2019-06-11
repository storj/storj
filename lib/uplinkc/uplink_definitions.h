#pragma once

#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

typedef struct APIKey  { long _handle; } APIKey;
typedef struct Uplink  { long _handle; } Uplink;
typedef struct Project { long _handle; } Project;

typedef struct UplinkConfig {
    struct {
        struct {
            uint8_t SkipPeerCAWhitelist;
        } TLS;
    } Volatile;
} UplinkConfig;