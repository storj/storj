// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdbool.h>
#include <stdint.h>
#include <time.h>

typedef __SIZE_TYPE__ GoUintptr;
typedef int64_t Size;

typedef GoUintptr APIKeyRef_t;
typedef GoUintptr IDVersionRef_t;
typedef GoUintptr UplinkRef_t;

typedef struct IDVersion {
    uint16_t number;
} IDVersion_t;

