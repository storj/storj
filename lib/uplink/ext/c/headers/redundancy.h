// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

#include <stdint.h>

typedef uint8_t RedundancyAlgorithm;

struct RedundancyScheme
{
    RedundancyAlgorithm Algorithm;
    // TODO: should we use `memory.Size`/`Size`?
    int32_t ShareSize;
    int16_t RequiredShares;
    int16_t RepairShares;
    int16_t OptimalShares;
    int16_t TotalShares;
};
