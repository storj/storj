// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import {BUCKET_USAGE_MUTATIONS, PROJECT_USAGE_MUTATIONS} from '@/store/mutationConstants';
import {BUCKET_USAGE_ACTIONS, PROJECT_USAGE_ACTIONS} from '@/utils/constants/actionNames';
import { fetchBucketUsages, fetchProjectUsage } from '@/api/usage';

export const usageModule = {
    state: {
        projectUsage: {storage: 0, egress: 0, objectCount: 0} as ProjectUsage,
    },
    mutations: {
        [PROJECT_USAGE_MUTATIONS.FETCH](state: any, projectUsage: ProjectUsage) {
           state.projectUsage = projectUsage;
        },
        [PROJECT_USAGE_MUTATIONS.CLEAR](state:any) {
            state.projectUsage = {storage: 0, egress: 0, objectCount: 0} as ProjectUsage;
        }
    },
    actions: {
        [PROJECT_USAGE_ACTIONS.FETCH]: async function({commit, rootGetters}: any, {startDate, endDate}: any): Promise<RequestResponse<ProjectUsage>> {
            const projectID = rootGetters.selectedProject.id;

            let result = await fetchProjectUsage(projectID, startDate, endDate);

            if (result.isSuccess) {
                commit(PROJECT_USAGE_MUTATIONS.FETCH, result.data);
            }

            return result;
        },
        [PROJECT_USAGE_ACTIONS.CLEAR]: function({commit}): void {
           commit(PROJECT_USAGE_MUTATIONS.CLEAR);
        }
    }
};

export const bucketUsageModule = {
    state: {
        pages: [] as BucketUsagePage[],
        currentPage: {} as BucketUsagePage,
        cursor: {limit:2} as BucketUsageCursor
    },
    mutations: {
        [BUCKET_USAGE_MUTATIONS.FETCH](state: any, page: BucketUsagePage) {
            state.pages = [page] as BucketUsagePage[];
            state.currentPage = page;
            if (page.hasMore) {
                state.cursor.afterBucket = page.bucketUsages[page.bucketUsages.length-1];
            }
        },
        [BUCKET_USAGE_MUTATIONS.FETCH_NEXT](state: any, page: BucketUsagePage) {
            state.pages.push(page);
            state.currentPage = page;
            if (page.hasMore) {
                state.cursor.afterBucket = page.bucketUsages[page.bucketUsages.length-1];
            }
        },
        [BUCKET_USAGE_MUTATIONS.CLEAR](state: any) {
            state.pages = [] as BucketUsagePage[];
            state.cursor.afterBucket = '';
            state.currentPage = {} as BucketUsagePage;
        }
    },
    actions: {
        [BUCKET_USAGE_ACTIONS.FETCH]: async function({commit, rootGetters, state}: any, before: Date): Promise<RequestResponse<BucketUsagePage>> {
            const projectID = rootGetters.selectedProject.id;

            commit(BUCKET_USAGE_MUTATIONS.CLEAR);
            let result = await fetchBucketUsages(projectID, before, state.cursor);
            console.log("action fetch: ", result);
            if (result.isSuccess) {
                commit(BUCKET_USAGE_MUTATIONS.FETCH, result.data);
            }

            return result;
        },
        [BUCKET_USAGE_ACTIONS.FETCH_NEXT]: async function({commit, rootGetters}: any, {before, cursor}: any): Promise<RequestResponse<BucketUsagePage>> {
            const projectID = rootGetters.selectedProject.id;
            before = before as Date;
            cursor = cursor as BucketUsagePage;

            let result = await fetchBucketUsages(projectID, before, cursor);

            if (result.isSuccess) {
                commit(BUCKET_USAGE_MUTATIONS.FETCH_NEXT, result.data);
            }

            return result;
        },
        [BUCKET_USAGE_ACTIONS.CLEAR]: function({commit}) {
            commit(BUCKET_USAGE_MUTATIONS.CLEAR);
        }
    }
};
