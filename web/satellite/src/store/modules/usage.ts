// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import { DateRange, ProjectUsage, UsageApi } from '@/types/usage';

export const PROJECT_USAGE_ACTIONS = {
    FETCH: 'fetchProjectUsage',
    FETCH_CURRENT_ROLLUP: 'fetchCurrentProjectUsage',
    FETCH_PREVIOUS_ROLLUP: 'fetchPreviousProjectUsage',
    CLEAR: 'clearProjectUsage',
};

export const PROJECT_USAGE_MUTATIONS = {
    SET_PROJECT_USAGE: 'SET_PROJECT_USAGE',
    SET_DATE: 'SET_DATE_PROJECT_USAGE',
    CLEAR: 'CLEAR_PROJECT_USAGE',
};

const defaultState = new ProjectUsage(0, 0, 0, new Date(), new Date());

export class UsageState {
    public projectUsage: ProjectUsage = defaultState;
    public startDate: Date = new Date();
    public endDate: Date = new Date();
}

export function makeUsageModule(api: UsageApi): StoreModule<UsageState> {
    return {
        state: new UsageState(),
        mutations: {
            [PROJECT_USAGE_MUTATIONS.SET_PROJECT_USAGE](state: UsageState, projectUsage: ProjectUsage) {
                state.projectUsage = projectUsage;
            },
            [PROJECT_USAGE_MUTATIONS.SET_DATE](state: UsageState, dateRange: DateRange) {
                state.startDate = dateRange.startDate;
                state.endDate = dateRange.endDate;
            },
            [PROJECT_USAGE_MUTATIONS.CLEAR](state: UsageState) {
                state.projectUsage = defaultState;
                state.startDate = new Date();
                state.endDate = new Date();
            },
        },
        actions: {
            [PROJECT_USAGE_ACTIONS.FETCH]: async function({commit, rootGetters}: any, dateRange: DateRange): Promise<ProjectUsage> {
                const now = new Date();
                let beforeUTC = new Date(Date.UTC(dateRange.endDate.getFullYear(), dateRange.endDate.getMonth(), dateRange.endDate.getDate(), 23, 59));

                if (now.getUTCFullYear() === dateRange.endDate.getUTCFullYear() &&
                    now.getUTCMonth() === dateRange.endDate.getUTCMonth() &&
                    now.getUTCDate() <= dateRange.endDate.getUTCDate()) {
                    beforeUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), now.getMinutes()));
                }

                const sinceUTC = new Date(Date.UTC(dateRange.startDate.getFullYear(), dateRange.startDate.getMonth(), dateRange.startDate.getDate()));

                const usage: ProjectUsage = await api.get(rootGetters.selectedProject.id, sinceUTC, beforeUTC);

                commit(PROJECT_USAGE_MUTATIONS.SET_DATE, new DateRange(sinceUTC, beforeUTC));
                commit(PROJECT_USAGE_MUTATIONS.SET_PROJECT_USAGE, usage);

                return usage;
            },
            [PROJECT_USAGE_ACTIONS.FETCH_CURRENT_ROLLUP]: async function({commit, rootGetters}: any): Promise<ProjectUsage> {
                const projectID: string = rootGetters.selectedProject.id;

                const now = new Date();
                const endUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), now.getUTCHours(), now.getMinutes()));
                const startUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 1));

                const usage: ProjectUsage = await api.get(projectID, startUTC, endUTC);

                commit(PROJECT_USAGE_MUTATIONS.SET_DATE, new DateRange(startUTC, endUTC));
                commit(PROJECT_USAGE_MUTATIONS.SET_PROJECT_USAGE, usage);

                return usage;
            },
            [PROJECT_USAGE_ACTIONS.FETCH_PREVIOUS_ROLLUP]: async function({commit, rootGetters}: any): Promise<ProjectUsage> {
                const projectID = rootGetters.selectedProject.id;

                const now = new Date();
                const startUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth() - 1, 1));
                const endUTC = new Date(Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), 0, 23, 59, 59));

                const usage: ProjectUsage = await api.get(projectID, startUTC, endUTC);

                commit(PROJECT_USAGE_MUTATIONS.SET_DATE, new DateRange(startUTC, endUTC));
                commit(PROJECT_USAGE_MUTATIONS.SET_PROJECT_USAGE, usage);

                return usage;
            },
            [PROJECT_USAGE_ACTIONS.CLEAR]: function({commit}): void {
                commit(PROJECT_USAGE_MUTATIONS.CLEAR);
            },
        },
    };
}
