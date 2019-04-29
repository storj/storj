// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include "uplink-cgo-common.h"

typedef struct Uplink {
    GoUintptr tc;
    struct Config *cfg;
    char *error;
};
