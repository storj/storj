// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdbool.h>
#include <stdint.h>
#include "../pb/uplink.pb-c.h"

typedef __SIZE_TYPE__ GoUintptr;
typedef int64_t Size;

typedef GoUintptr APIKeyRef_t;
typedef GoUintptr IDVersionRef_t;
typedef GoUintptr UplinkRef_t;
typedef GoUintptr UplinkConfigRef_t;
typedef GoUintptr ProjectRef_t;
typedef GoUintptr BucketRef_t;
typedef GoUintptr BucketConfigRef_t;

// Protobuf aliases
typedef Storj__Libuplink__IDVersion IDVersion_t;
typedef Storj__Libuplink__UplinkConfig UplinkConfig_t;
typedef Storj__Libuplink__TLSConfig TLSConfig_t;
typedef Storj__Libuplink__BucketConfig BucketConfig_t;
typedef Storj__Libuplink__RedundancyScheme RedundancyScheme_t;
typedef Storj__Libuplink__EncryptionParameters EncryptionParameters_t;
typedef Storj__Libuplink__Bucket Bucket_t;
