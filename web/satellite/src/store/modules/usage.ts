// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { PROJECT_USAGE_MUTATIONS } from '@/store/mutationConstants';
import { PROJECT_USAGE_ACTIONS } from '@/utils/constants/actionNames';
import { fetchProjectUsage } from '@/api/usage';
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
