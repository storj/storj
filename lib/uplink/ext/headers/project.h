// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

typedef __SIZE_TYPE__ GoUintptr;
typedef uint8_t Key[32];

struct Project {
    GoUintptr GoProject;
};

struct ProjectOptionsVolatile {
    Key* EncryptionKey;
};

struct ProjectOptions {
    struct ProjectOptionsVolatile Volatile;
};