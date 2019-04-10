// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { PROJECT_USAGE_MUTATIONS } from '@/store/mutationConstants';
import { PROJECT_USAGE_ACTIONS } from '@/utils/constants/actionNames';
import { fetchProjectUsage } from '@/api/usage';

export const usageModule = {
    state: {
        projectUsage: {storage: 0, egress: 0, objectCount: 0} as ProjectUsage
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
