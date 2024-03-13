// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { Bucket, BucketCursor, BucketPage, BucketPlacement, BucketsApi } from '@/types/buckets';
import { HttpClient } from '@/utils/httpClient';
import { APIError } from '@/utils/error';
import { getVersioning } from '@/types/versioning';

/**
 * BucketsHttpApi is an HTTP implementation of the Buckets API.
 * Exposes all bucket-related functionality.
 */
export class BucketsHttpApi implements BucketsApi {
    private readonly client: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/v0/buckets';

    /**
     * Fetch buckets.
     *
     * @returns BucketPage
     * @throws Error
     */
    public async get(projectID: string, before: Date, cursor: BucketCursor): Promise<BucketPage> {
        const paramsString = Object.entries({
            projectID,
            before: before.toISOString(),
            limit: cursor.limit,
            search: encodeURIComponent(cursor.search),
            page: cursor.page,
        }).map(entry => entry.join('=')).join('&');

        const path = `${this.ROOT_PATH}/usage-totals?${paramsString}`;
        const response = await this.client.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Cannot get buckets',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const result = await response.json();

        return new BucketPage(
            result.bucketUsages?.map(usage =>
                new Bucket(
                    usage.bucketName,
                    getVersioning(usage.versioning),
                    usage.defaultPlacement,
                    usage.location,
                    usage.storage,
                    usage.egress,
                    usage.objectCount,
                    usage.segmentCount,
                    new Date(usage.since),
                    new Date(usage.before),
                ),
            ) || [],
            result.search,
            result.limit,
            result.offset,
            result.pageCount,
            result.currentPage,
            result.totalCount,
        );
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
     * Fetch all bucket placements.
     *
     * @returns BucketPlacement[]
     * @throws Error
     */
    public async getAllBucketPlacements(projectId: string): Promise<BucketPlacement[]> {
        const path = `${this.ROOT_PATH}/bucket-placements?publicID=${projectId}`;
        const response = await this.client.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get bucket placements',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const result = await response.json();

        return result ? result : [];
    }
}
