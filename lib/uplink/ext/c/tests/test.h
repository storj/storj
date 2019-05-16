// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdbool.h>
#include <stdint.h>

typedef __SIZE_TYPE__ GoUintptr;

struct Simple {
    char *Str1;
    int32_t Int2;
    uint32_t Uint3;
};

struct Nested {
    struct Simple Simple;
    int Int4;
};

struct NestedPointer {
    GoUintptr Pointer;
    struct Simple Simple;
};
