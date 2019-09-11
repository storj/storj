// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Exposes all bucket-related functionality
 */
export interface BucketsApi {
    /**
     * Fetch buckets
     *
     * @returns BucketPage
     * @throws Error
     */
    get(projectId: string, before: Date, cursor: BucketCursor): Promise<BucketPage>;
}

/**
 * Bucket class holds info for Bucket entity.
 */
export class Bucket {
    public bucketName: string;
    public storage: number;
    public egress: number;
    public objectCount: number;
    public since: Date;
    public before: Date;

    constructor(bucketName: string = '', storage: number = 0, egress: number = 0, objectCount: number = 0, since: Date = new Date(), before: Date = new Date()) {
        this.bucketName = bucketName;
        this.storage = storage;
        this.egress = egress;
        this.objectCount = objectCount;
        this.since = since;
        this.before = before;
    }
}

/**
 * BucketPage class holds bucket total usages and flag whether more usages available.
 */
export class BucketPage {
    buckets: Bucket[];
    search: string;
    limit: number;
    offset: number;
    pageCount: number;
    currentPage: number;
    totalCount: number;

    constructor(buckets: Bucket[] = [], search: string = '', limit: number = 0, offset: number = 0, pageCount: number = 0, currentPage: number = 0, totalCount: number = 0) {
        this.buckets = buckets;
        this.search = search;
        this.limit = limit;
        this.offset = offset;
        this.pageCount = pageCount;
        this.currentPage = currentPage;
        this.totalCount = totalCount;
    }
}

/**
 * BucketCursor class holds cursor for bucket name and limit.
 */
export class BucketCursor {
    search: string;
    limit: number;
    page: number;

    constructor(search: string = '', limit: number = 0, page: number = 0) {
        this.search = search;
        this.limit = limit;
        this.page = page;
    }
}
