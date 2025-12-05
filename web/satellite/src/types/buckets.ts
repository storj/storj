// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';
import { Placement } from '@/types/placements';
import { Versioning } from '@/types/versioning';
import { COMPLIANCE_LOCK, GOVERNANCE_LOCK, NO_MODE_SET, ObjLockMode } from '@/types/objectLock';

/**
 * Exposes all bucket-related functionality.
 */
export interface BucketsApi {
    /**
     * Fetch buckets
     *
     * @returns BucketPage
     * @throws Error
     */
    get(projectId: string, since: Date, before: Date, cursor: BucketCursor): Promise<BucketPage>;

    /**
     * Fetch single bucket data.
     *
     * @returns Bucket
     * @throws Error
     */
    getSingle(projectId: string, bucketName: string, before: Date): Promise<Bucket>;

    /**
     * Fetch all bucket names
     *
     * @returns string[]
     * @throws Error
     */
    getAllBucketNames(projectId: string): Promise<string[]>;

    /**
     *
     * Fetch all bucket metadata
     *
     * @returns BucketMetadata[]
     * @throws Error
     */
    getAllBucketMetadata(projectId: string): Promise<BucketMetadata[]>

    /**
     * Fetch placement details
     *
     * @returns PlacementDetails[]
     * @throws Error
     */
    getPlacementDetails(projectID: string): Promise<PlacementDetails[]>;
}

/**
 * Bucket class holds info for Bucket entity.
 */
export class Bucket {
    public defaultRetentionMode: ObjLockMode | typeof NO_MODE_SET = NO_MODE_SET;

    public constructor(
        public name: string = '',
        public versioning: Versioning = Versioning.NotSupported,
        public objectLockEnabled: boolean = false,
        public defaultPlacement: number = 0,
        public location: string = '',
        public storage: number = 0,
        public egress: number = 0,
        public objectCount: number = 0,
        public segmentCount: number = 0,
        public since: Date = new Date(),
        public before: Date = new Date(),
        public _defaultRetentionMode: number = 0,
        public defaultRetentionDays: number | null = null,
        public defaultRetentionYears: number | null = null,
        public createdAt: Date = new Date(),
        public creatorEmail: string = '',
    ) {
        if (this._defaultRetentionMode) {
            this.defaultRetentionMode = this._defaultRetentionMode === 1 ? COMPLIANCE_LOCK : GOVERNANCE_LOCK;
        }
    }
}

/**
 * BucketPage class holds bucket total usages and flag whether more usages available.
 */
export class BucketPage {
    public constructor(
        public buckets: Bucket[] = [],
        public search: string = '',
        public limit: number = 0,
        public offset: number = 0,
        public pageCount: number = 0,
        public currentPage: number = 0,
        public totalCount: number = 0,
    ) { }
}

/**
 * BucketCursor class holds cursor for bucket name and limit.
 */
export class BucketCursor {
    public constructor(
        public search: string = '',
        public limit: number = DEFAULT_PAGE_LIMIT,
        public page: number = 1,
    ) { }
}

/**
 * BucketMeta class holds misc bucket metadata.
 */
export class BucketMetadata {
    public constructor(
        public name: string = '',
        public versioning: Versioning = Versioning.NotSupported,
        public placement: Placement = new Placement(),
        public objectLockEnabled: boolean = false,
    ) { }
}

export class PlacementDetails {
    public constructor(
        public id: number = 0,
        public idName: string = '',
        public name: string = '',
        public title: string = '',
        public description: string = '',
        public pending: boolean = false,
        public shortName: string = '',
        public lucideIcon: string = '',
    ) { }
}