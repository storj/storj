// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import { Bucket, BucketCursor, BucketPage, BucketsApi } from '@/types/buckets';
import { HttpClient } from '@/utils/httpClient';
import { APIError } from '@/utils/error';

/**
 * BucketsApiGql is a graphql implementation of Buckets API.
 * Exposes all bucket-related functionality.
 */
export class BucketsApiGql extends BaseGql implements BucketsApi {
    private readonly client: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/buckets';

    /**
     * Fetch buckets.
     *
     * @returns BucketPage
     * @throws Error
     */
    public async get(projectId: string, before: Date, cursor: BucketCursor): Promise<BucketPage> {
        const query =
            `query($projectId: String!, $before: DateTime!, $limit: Int!, $search: String!, $page: Int!) {
                project(publicId: $projectId) {
                    bucketUsages(before: $before, cursor: {
                        limit: $limit, search: $search, page: $page
                    }) {
                        bucketUsages {
                            bucketName,
                            storage,
                            egress,
                            objectCount,
                            segmentCount,
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

    /**
     * Fetch all bucket names.
     *
     * @returns string[]
     * @throws Error
     */
    public async getAllBucketNames(projectId: string): Promise<string[]> {
        const path = `${this.ROOT_PATH}/bucket-names?publicID=${projectId}`;
        const response = await this.client.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get bucket names',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const result = await response.json();

        return result ? result : [];
    }

    /**
     * Method for mapping buckets page from json to BucketPage type.
     *
     * @param page anonymous object from json
     */
    private getBucketPage(page: any): BucketPage { // eslint-disable-line @typescript-eslint/no-explicit-any
        if (!page) {
            return new BucketPage();
        }

        const buckets: Bucket[] = page.bucketUsages.map(key =>
            new Bucket(
                key.bucketName,
                key.storage,
                key.egress,
                key.objectCount,
                key.segmentCount,
                new Date(key.since),
                new Date(key.before)));

        return new BucketPage(buckets, page.search, page.limit, page.offset, page.pageCount, page.currentPage, page.totalCount);
    }
}
