// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdbool.h>
#include <stdint.h>
#include "../pb/uplink.pb-c.h"

typedef __SIZE_TYPE__ GoUintptr;
typedef int64_t Size;

typedef GoUintptr API_Key_Ref;
typedef GoUintptr IDVersionRef;
typedef GoUintptr UplinkRef;
typedef GoUintptr UplinkConfigRef;
typedef GoUintptr ProjectRef;
typedef GoUintptr BucketRef;
typedef GoUintptr BucketConfigRef;

// Protobuf aliases
typedef Storj__Libuplink__IDVersion pbIDVersion;
typedef Storj__Libuplink__UplinkConfig pbUplinkConfig;
typedef Storj__Libuplink__TLSConfig pbTLSConfig;
typedef Storj__Libuplink__ProjectOptions pbProjectOptions;
typedef Storj__Libuplink__BucketConfig pbBucketConfig;
typedef Storj__Libuplink__RedundancyScheme pbRedundancyScheme;
typedef Storj__Libuplink__EncryptionParameters pbEncryptionParameters;
typedef Storj__Libuplink__Bucket pbBucket;

void *get_snapshot(struct GoValue *, char **);
void protoToGoValue(void *, struct GoValue *, char **);
