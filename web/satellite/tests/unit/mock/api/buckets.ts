// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BucketCursor, BucketPage, BucketsApi } from '@/types/buckets';

/**
 * Mock for BucketsApi
 */
export class BucketsMock implements BucketsApi {
    get(_projectId: string, _before: Date, _cursor: BucketCursor): Promise<BucketPage> {
        return Promise.resolve(new BucketPage());
    }

    getAllBucketNames(_projectId: string): Promise<string[]> {
        return Promise.resolve(['test']);
    }
}
