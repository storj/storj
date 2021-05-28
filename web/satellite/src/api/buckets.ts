// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BaseGql } from '@/api/baseGql';
import { ErrorUnauthorized } from '@/api/errors/ErrorUnauthorized';
import { Bucket, BucketCursor, BucketPage, BucketsApi } from '@/types/buckets';
import { HttpClient } from '@/utils/httpClient';

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

    /**
     * Fetch all bucket names.
     *
     * @returns string[]
     * @throws Error
     */
    public async getAllBucketNames(projectId: string): Promise<string[]> {
        const path = `${this.ROOT_PATH}/bucket-names?projectID=${projectId}`;
        const response = await this.client.get(path);

        if (!response.ok) {
            if (response.status === 401) {
                throw new ErrorUnauthorized();
            }

            throw new Error('Can not get bucket names');
        }

        const result = await response.json();

        return result ? result : [];
    }

    /**
     * Method for mapping buckets page from json to BucketPage type.
     *
     * @param page anonymous object from json
     */
    private getBucketPage(page: any): BucketPage {
        if (!page) {
            return new BucketPage();
        }

        const buckets: Bucket[] = page.bucketUsages.map(key =>
            new Bucket(
                key.bucketName,
                key.storage,
                key.egress,
                key.objectCount,
                new Date(key.since),
                new Date(key.before)));

        return new BucketPage(buckets, page.search, page.limit, page.offset, page.pageCount, page.currentPage, page.totalCount);
    }
}
