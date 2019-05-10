// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

typedef __SIZE_TYPE__ GoUintptr;
typedef uint8_t Key[32];

typedef GoUintptr Project;

struct ProjectOptions
{
    struct
    {
        Key *EncryptionKey;

    } Volatile;
};

// TODO: move to bucket.h? (also move go src)
struct BucketConfig
{
    CipherSuite PathCipher;
};