// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import { Bucket, BucketCursor, BucketPage, BucketsApi } from '@/types/buckets';

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

        return this.getBucketPage(response.data.project.bucketUsages);
    }

    private getBucketPage(page: any): BucketPage {
        if (!page) {
            return new BucketPage();
        }

        const bucketPage: BucketPage = new BucketPage();

        bucketPage.buckets = page.bucketUsages.map(key => new Bucket(key.bucketName, key.storage, key.egress, key.objectCount, key.since, key.before));

        bucketPage.search = page.search;
        bucketPage.limit = page.limit;
        bucketPage.offset = page.offset;
        bucketPage.pageCount = page.pageCount;
        bucketPage.currentPage = page.currentPage;
        bucketPage.totalCount = page.totalCount;

        return bucketPage;
    }
}
