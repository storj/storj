// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdint.h>

// TODO: convert to enum
typedef uint8_t Cipher;
// TODO: convert to enum
typedef uint8_t CipherSuite;

struct EncryptionParameters
{
    CipherSuite CipherSuite;
    int32_t BlockSize;
};
