// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    Bucket,
    BucketCursor,
    BucketMetadata,
    BucketPage,
    BucketsApi, PlacementDetails,
} from '@/types/buckets';
import { HttpClient } from '@/utils/httpClient';
import { APIError } from '@/utils/error';
import { getVersioning } from '@/types/versioning';
import { Placement } from '@/types/placements.js';

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
    public async get(projectID: string, since: Date, before: Date, cursor: BucketCursor): Promise<BucketPage> {
        const paramsString = Object.entries({
            projectID,
            since: since.toISOString(),
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
                    usage.objectLockEnabled,
                    usage.defaultPlacement,
                    usage.location,
                    usage.storage,
                    usage.egress,
                    usage.objectCount,
                    usage.segmentCount,
                    new Date(usage.since),
                    new Date(usage.before),
                    usage.defaultRetentionMode,
                    usage.defaultRetentionDays,
                    usage.defaultRetentionYears,
                    new Date(usage.createdAt),
                    usage.creatorEmail,
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
     * Fetch single bucket data.
     *
     * @returns Bucket
     * @throws Error
     */
    public async getSingle(projectID: string, bucketName: string, before: Date): Promise<Bucket> {
        const paramsString = new URLSearchParams({
            projectID,
            bucket: bucketName,
            before: before.toISOString(),
        }).toString();

        const path = `${this.ROOT_PATH}/bucket-totals?${paramsString}`;
        const response = await this.client.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Cannot get single bucket data',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const result = await response.json();

        return new Bucket(
            result.bucketName,
            getVersioning(result.versioning),
            result.objectLockEnabled,
            result.defaultPlacement,
            result.location,
            result.storage,
            result.egress,
            result.objectCount,
            result.segmentCount,
            new Date(result.since),
            new Date(result.before),
            result.defaultRetentionMode,
            result.defaultRetentionDays,
            result.defaultRetentionYears,
            new Date(result.createdAt),
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
     * Fetch all bucket metadata.
     *
     * @returns BucketMetadata[]
     * @throws Error
     */
    public async getAllBucketMetadata(projectId: string): Promise<BucketMetadata[]> {
        const path = `${this.ROOT_PATH}/bucket-metadata?publicID=${projectId}`;
        const response = await this.client.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get bucket metadata',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const result = await response.json();

        return result?.map(bVersioning => new BucketMetadata(
            bVersioning.name,
            getVersioning(bVersioning.versioning),
            new Placement(
                bVersioning.placement.defaultPlacement,
                bVersioning.placement.location,
            ),
            bVersioning.objectLockEnabled,
        )) || [];
    }

    /**
     * Fetch placement details
     *
     * @returns PlacementDetails[]
     * @throws Error
     */
    public async getPlacementDetails(projectID: string): Promise<PlacementDetails[]> {
        const path = `${this.ROOT_PATH}/placement-details?projectID=${projectID}`;
        const response = await this.client.get(path);

        if (!response.ok) {
            throw new APIError({
                status: response.status,
                message: 'Can not get placement details',
                requestID: response.headers.get('x-request-id'),
            });
        }

        const result = await response.json();

        return result?.map(detail => new PlacementDetails(
            detail.id,
            detail.idName,
            detail.name,
            detail.title,
            detail.description,
            detail.pending,
        )) || [];
    }
}
