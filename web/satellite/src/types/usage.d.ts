// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// ProjectUsage sums usage for given period
declare type ProjectUsage = {
    storage: number,
    egress: number,
    objectCount: number,
    since: Date,
    before: Date,
}

// BucketUSage total usage of a bucket for given period
declare type BucketUsage = {
    bucketName: string,
    storage: number,
    egress: number,
    objectCount: number,
    since: Date,
    before: Date,
}

// BucketUsagePage holds bucket total usages and flag
// wether more usages available
declare type BucketUsagePage = {
    bucketUsages: BucketUsage[],
    search: string,
    limit: number,
    offset: number,
    pageCount: number,
    currentPage: number,
    totalCount: number,
}

// BucketUsageCursor holds cursor for bucket name and limit
declare type BucketUsageCursor = {
    search: string,
    limit: number,
    page: number,
}

declare type CreditUsage = {
    referred: number,
    usedCredits: number,
    availableCredits: number,
}
