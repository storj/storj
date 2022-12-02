// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

import { Bucket, BucketCursor, BucketPage, BucketsApi } from '@/types/buckets';
import { BucketsApiGql } from '@/api/buckets';
import { useProjectsStore } from '@/store/modules/projectsStore';

const BUCKETS_PAGE_LIMIT = 7;
const FIRST_PAGE = 1;

export class BucketsState {
    public allBucketNames: string[] = [];
    public cursor: BucketCursor = { limit: BUCKETS_PAGE_LIMIT, search: '', page: FIRST_PAGE };
    public page: BucketPage = { buckets: new Array<Bucket>(), currentPage: 1, pageCount: 1, offset: 0, limit: BUCKETS_PAGE_LIMIT, search: '', totalCount: 0 };
}

export const useBucketsStore = defineStore('buckets', () => {
    const bucketsState = reactive<BucketsState>({
        allBucketNames: [],
        cursor: { limit: BUCKETS_PAGE_LIMIT, search: '', page: FIRST_PAGE },
        page: { buckets: new Array<Bucket>(), currentPage: 1, pageCount: 1, offset: 0, limit: BUCKETS_PAGE_LIMIT, search: '', totalCount: 0 },
    });

    const api: BucketsApi = new BucketsApiGql();

    function setBucketsSearch(search: string): void {
        bucketsState.cursor.search = search;
    }

    function clearBucketsState(): void {
        bucketsState.allBucketNames = [];
        bucketsState.cursor = new BucketCursor('', BUCKETS_PAGE_LIMIT, FIRST_PAGE);
        bucketsState.page = new BucketPage([], '', BUCKETS_PAGE_LIMIT, 0, 1, 1, 0);
    }

    async function fetchBuckets(page: number): Promise<void> {
        const { projectsStore } = useProjectsStore();
        const projectID = projectsStore.selectedProject.id;
        const before = new Date();
        bucketsState.cursor.page = page;

        bucketsState.page = await api.get(projectID, before, bucketsState.cursor);
    }

    async function fetchAllBucketsNames(): Promise<void> {
        const { projectsStore } = useProjectsStore();
        const projectID = projectsStore.selectedProject.id;

        bucketsState.allBucketNames = await api.getAllBucketNames(projectID);
    }

    return {
        bucketsState,
        setBucketsSearch,
        clearBucketsState,
        fetchBuckets,
        fetchAllBucketsNames,
    };
});
