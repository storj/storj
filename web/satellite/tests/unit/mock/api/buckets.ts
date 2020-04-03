// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BucketCursor, BucketPage, BucketsApi } from '@/types/buckets';

/**
 * Mock for BucketsApi
 */
export class BucketsMock implements BucketsApi {
    get(projectId: string, before: Date, cursor: BucketCursor): Promise<BucketPage> {
        return Promise.resolve(new BucketPage());
    }
}
