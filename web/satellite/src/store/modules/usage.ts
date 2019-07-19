// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { BUCKET_USAGE_MUTATIONS, PROJECT_USAGE_MUTATIONS, CREDIT_USAGE_MUTATIONS } from '@/store/mutationConstants';
import { BUCKET_USAGE_ACTIONS, PROJECT_USAGE_ACTIONS, CREDIT_USAGE_ACTIONS } from '@/utils/constants/actionNames';
import { fetchBucketUsages, fetchProjectUsage, fetchCreditUsage } from '@/api/usage';
import { RequestResponse } from '@/types/response';

export const usageModule = {
    state: {
        projectUsage: {storage: 0, egress: 0, objectCount: 0} as ProjectUsage,
        startDate: new Date(),
        endDate: new Date()
    },
    mutations: {
        [PROJECT_USAGE_MUTATIONS.FETCH](state: any, projectUsage: ProjectUsage) {
           state.projectUsage = projectUsage;
        },
        // TODO: create type here instead of {startDate, endDate}
        [PROJECT_USAGE_MUTATIONS.SET_DATE](state: any, {startDate, endDate}: any) {
            state.startDate = startDate as Date;
            state.endDate = endDate as Date;
        },
        [PROJECT_USAGE_MUTATIONS.CLEAR](state:any) {
            state.projectUsage = {storage: 0, egress: 0, objectCount: 0} as ProjectUsage;
            state.startDate = new Date();
            state.endData = new Date();
        }
    },
    actions: {
        [PROJECT_USAGE_ACTIONS.FETCH]: async function({commit, rootGetters}: any, {startDate, endDate}: any): Promise<RequestResponse<ProjectUsage>> {
            const projectID = rootGetters.selectedProject.id;

            let result: RequestResponse<ProjectUsage> = await fetchProjectUsage(projectID, startDate, endDate);

            if (result.isSuccess) {
                commit(PROJECT_USAGE_MUTATIONS.SET_DATE, {startDate, endDate});
                commit(PROJECT_USAGE_MUTATIONS.FETCH, result.data);
            }

            return result;
        },
        [PROJECT_USAGE_ACTIONS.FETCH_CURRENT_ROLLUP]: async function({commit, rootGetters}: any): Promise<RequestResponse<ProjectUsage>> {
            const projectID: string = rootGetters.selectedProject.id;

            const endDate = new Date();
            const startDate = new Date(Date.UTC(endDate.getUTCFullYear(), endDate.getUTCMonth(), 1));

            let result: RequestResponse<ProjectUsage> = await fetchProjectUsage(projectID, startDate, endDate);

            if (result.isSuccess) {
                commit(PROJECT_USAGE_MUTATIONS.SET_DATE, {startDate, endDate});
                commit(PROJECT_USAGE_MUTATIONS.FETCH, result.data);
            }

            return result;
        },
        [PROJECT_USAGE_ACTIONS.FETCH_PREVIOUS_ROLLUP]: async function({commit, rootGetters}: any): Promise<RequestResponse<ProjectUsage>> {
            const projectID = rootGetters.selectedProject.id;

            const date = new Date();
            const startDate = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() - 1, 1));
            const endDate = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), 0, 23, 59, 59));

            let result = await fetchProjectUsage(projectID, startDate, endDate);

            if (result.isSuccess) {
                commit(PROJECT_USAGE_MUTATIONS.SET_DATE, {startDate, endDate});
                commit(PROJECT_USAGE_MUTATIONS.FETCH, result.data);
            }

            return result;
        },
        [PROJECT_USAGE_ACTIONS.CLEAR]: function({commit}): void {
            commit(PROJECT_USAGE_MUTATIONS.CLEAR);
        }
    }
};

const bucketPageLimit = 6;
const firstPage = 1;

export const bucketUsageModule = {
    state: {
        cursor: { limit: bucketPageLimit, search: '', page: firstPage } as BucketUsageCursor,
        page: { bucketUsages: [] as BucketUsage[] } as BucketUsagePage,
        totalCount: 0,
    },
    mutations: {
        [BUCKET_USAGE_MUTATIONS.FETCH](state: any, page: BucketUsagePage) {
            state.page = page;

            if (page.totalCount > 0) {
                state.totalCount = page.totalCount;
            }
        },
        [BUCKET_USAGE_MUTATIONS.SET_PAGE](state: any, page: number) {
           state.cursor.page = page;
        },
        [BUCKET_USAGE_MUTATIONS.SET_SEARCH](state: any, search: string) {
            state.cursor.search = search;
        },
        [BUCKET_USAGE_MUTATIONS.CLEAR](state: any) {
            state.cursor = { limit: bucketPageLimit, search: '', page: firstPage } as BucketUsageCursor;
            state.page = { bucketUsages: [] as BucketUsage[] } as BucketUsagePage;
            state.totalCount = 0;
        }
    },
    actions: {
        [BUCKET_USAGE_ACTIONS.FETCH]: async function({commit, rootGetters, state}: any, page: number): Promise<RequestResponse<BucketUsagePage>> {
            const projectID = rootGetters.selectedProject.id;
            const before = new Date();
            state.cursor.page = page;

            commit(BUCKET_USAGE_MUTATIONS.SET_PAGE, page);

            let result = await fetchBucketUsages(projectID, before, state.cursor);
            if (result.isSuccess) {
                commit(BUCKET_USAGE_MUTATIONS.FETCH, result.data);
            }

            return result;
        },
        [BUCKET_USAGE_ACTIONS.SET_SEARCH]: function({commit}, search: string) {
            commit(BUCKET_USAGE_MUTATIONS.SET_SEARCH, search);
        },
        [BUCKET_USAGE_ACTIONS.CLEAR]: function({commit}) {
            commit(BUCKET_USAGE_MUTATIONS.CLEAR);
        }
    }
};

export const creditUsageModule = {
    state: {
        creditUsage: { referred: 0, usedCredits: 0, availableCredits: 0 } as CreditUsage
    },
    mutations: {
        [CREDIT_USAGE_ACTIONS.FETCH](state: any, creditUsage: CreditUsage) {
            state.creditUsage = creditUsage;
        }
    },
    actions: {
        [CREDIT_USAGE_ACTIONS.FETCH]: async function({commit, rootGetters}: any): Promise<RequestResponse<CreditUsage>> {
            let result = await fetchCreditUsage();

            if (result.isSuccess) {
                commit(CREDIT_USAGE_MUTATIONS.FETCH, result.data);
            }

            return result;
        },
    }
};
