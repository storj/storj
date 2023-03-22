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
    const state = reactive<BucketsState>(new BucketsState());

    const api: BucketsApi = new BucketsApiGql();

    function setBucketsSearch(search: string): void {
        state.cursor.search = search;
    }

    function clearBucketsState(): void {
        state.allBucketNames = [];
        state.cursor = new BucketCursor('', BUCKETS_PAGE_LIMIT, FIRST_PAGE);
        state.page = new BucketPage([], '', BUCKETS_PAGE_LIMIT, 0, 1, 1, 0);
    }

    async function fetchBuckets(page: number): Promise<void> {
        const { projectsState } = useProjectsStore();
        const projectID = projectsState.selectedProject.id;
        const before = new Date();
        state.cursor.page = page;

        state.page = await api.get(projectID, before, state.cursor);
    }

    async function fetchAllBucketsNames(): Promise<void> {
        const { projectsState } = useProjectsStore();
        const projectID = projectsState.selectedProject.id;

        state.allBucketNames = await api.getAllBucketNames(projectID);
    }

    return {
        bucketsState: state,
        setBucketsSearch,
        clearBucketsState,
        fetchBuckets,
        fetchAllBucketsNames,
    };
});
