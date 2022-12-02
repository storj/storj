// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { Bucket, BucketCursor, BucketPage, BucketsApi } from '@/types/buckets';
import { StoreModule } from '@/types/store';

export const BUCKET_ACTIONS = {
    FETCH: 'setBuckets',
    FETCH_ALL_BUCKET_NAMES: 'getAllBucketNames',
    SET_SEARCH: 'setBucketSearch',
    CLEAR: 'clearBuckets',
};

export const BUCKET_MUTATIONS = {
    SET: 'setBuckets',
    SET_ALL_BUCKET_NAMES: 'setAllBucketNames',
    SET_SEARCH: 'setBucketSearch',
    SET_PAGE: 'setBucketPage',
    CLEAR: 'clearBuckets',
};

const {
    FETCH,
    FETCH_ALL_BUCKET_NAMES,
} = BUCKET_ACTIONS;
const {
    SET,
    SET_ALL_BUCKET_NAMES,
    SET_PAGE,
    SET_SEARCH,
    CLEAR,
} = BUCKET_MUTATIONS;
const bucketPageLimit = 7;
const firstPage = 1;

export class BucketsState {
    public allBucketNames: string[] = [];
    public cursor: BucketCursor = { limit: bucketPageLimit, search: '', page: firstPage };
    public page: BucketPage = { buckets: new Array<Bucket>(), currentPage: 1, pageCount: 1, offset: 0, limit: bucketPageLimit, search: '', totalCount: 0 };
}

interface BucketsContext {
    state: BucketsState
    commit: (string, ...unknown) => void
    rootGetters: {
        selectedProject: {
            id: string
        }
    }
}

/**
 * creates buckets module with all dependencies
 *
 * @param api - buckets api
 */
export function makeBucketsModule(api: BucketsApi): StoreModule<BucketsState, BucketsContext> {
    return {
        state: new BucketsState(),

        mutations: {
            [SET](state: BucketsState, page: BucketPage) {
                state.page = page;
            },
            [SET_ALL_BUCKET_NAMES](state: BucketsState, allBucketNames: string[]) {
                state.allBucketNames = allBucketNames;
            },
            [SET_PAGE](state: BucketsState, page: number) {
                state.cursor.page = page;
            },
            [SET_SEARCH](state: BucketsState, search: string) {
                state.cursor.search = search;
            },
            [CLEAR](state: BucketsState) {
                state.allBucketNames = [];
                state.cursor = new BucketCursor('', bucketPageLimit, firstPage);
                state.page = new BucketPage([], '', bucketPageLimit, 0, 1, 1, 0);
            },
        },
        actions: {
            [FETCH]: async function({ commit, rootGetters, state }: BucketsContext, page: number): Promise<BucketPage> {
                const projectID = rootGetters.selectedProject.id;
                const before = new Date();
                state.cursor.page = page;

                commit(SET_PAGE, page);

                const result: BucketPage = await api.get(projectID, before, state.cursor);

                commit(SET, result);

                return result;
            },
            [FETCH_ALL_BUCKET_NAMES]: async function({ commit, rootGetters }: BucketsContext): Promise<string[]> {
                const result: string[] = await api.getAllBucketNames(rootGetters.selectedProject.id);

                commit(SET_ALL_BUCKET_NAMES, result);

                return result;
            },
            [BUCKET_ACTIONS.SET_SEARCH]: function({ commit }: BucketsContext, search: string) {
                commit(SET_SEARCH, search);
            },
            [BUCKET_ACTIONS.CLEAR]: function({ commit }: BucketsContext) {
                commit(CLEAR);
            },
        },
        getters: {
            page: (state: BucketsState): BucketPage => state.page,
            cursor: (state: BucketsState): BucketCursor => state.cursor,
        },
    };
}
