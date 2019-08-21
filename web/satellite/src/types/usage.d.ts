// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// ProjectUsage sums usage for given period
declare type ProjectUsage = {
    storage: number,
    egress: number,
    objectCount: number,
    since: Date,
    before: Date,
};
