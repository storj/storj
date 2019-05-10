// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdint.h>

struct Bucket
{
    char* Name;
    uint64_t Created;
    Cipher PathCipher;
    // TODO: should we use `memory.Size`/`Size` for this?
    int64_t SegmentSize;
    struct RedundancyScheme RedundancyScheme;
    struct EncryptionParameters EncryptionParameters;
};