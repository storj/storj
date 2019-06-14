#pragma once

#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>

typedef struct APIKey   { long _handle; } APIKeyRef_t;
typedef struct Uplink   { long _handle; } UplinkRef_t;
typedef struct Project  { long _handle; } ProjectRef_t;

typedef struct UplinkConfig {
    struct {
        struct {
            bool SkipPeerCAWhitelist;
        } TLS;
    } Volatile;
} UplinkConfig_t;
