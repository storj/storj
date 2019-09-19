// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import { BucketCursor, BucketPage, BucketsApi } from '@/types/buckets';

/**
 * BucketsApiGql is a graphql implementation of Buckets API.
 * Exposes all bucket-related functionality
 */
export class BucketsApiGql extends BaseGql implements BucketsApi {
    /**
     * Fetch buckets
     *
     * @returns BucketPage
     * @throws Error
     */
    public async get(projectId: string, before: Date, cursor: BucketCursor): Promise<BucketPage> {
        const query =
            `query($projectId: String!, $before: DateTime!, $limit: Int!, $search: String!, $page: Int!) {
                project(id: $projectId) {
                    bucketUsages(before: $before, cursor: {
                        limit: $limit, search: $search, page: $page
                    }) {
                        bucketUsages {
                            bucketName,
                            storage,
                            egress,
                            objectCount,
                            since,
                            before
                        },
                        search,
                        limit,
                        offset,
                        pageCount,
                        currentPage,
                        totalCount
                    }
                }
            }`;

        const variables = {
            projectId,
            before: before.toISOString(),
            limit: cursor.limit,
            search: cursor.search,
            page: cursor.page,
        };

        const response = await this.query(query, variables);

        return this.fromJson(response.data.project.bucketUsages);
    }

    private fromJson(bucketPage): BucketPage {
        return new BucketPage(bucketPage.bucketUsages, bucketPage.search, bucketPage.limit, bucketPage.offset, bucketPage.pageCount, bucketPage.currentPage, bucketPage.totalCount);
    }
}
