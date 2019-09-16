// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { ProjectUsageApiGql } from '@/api/usage';
import { StoreModule } from '@/store';
import { DateRange, ProjectUsage } from '@/types/usage';

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

class UsageState {
    public projectUsage: ProjectUsage = defaultState;
    public startDate: Date = new Date();
    public endDate: Date = new Date();
}

export function makeUsageModule(api: ProjectUsageApiGql): StoreModule<UsageState> {
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
                const projectID = rootGetters.selectedProject.id;

                const usage: ProjectUsage = await api.get(projectID, dateRange.startDate, dateRange.endDate);

                commit(PROJECT_USAGE_MUTATIONS.SET_DATE, dateRange);
                commit(PROJECT_USAGE_MUTATIONS.SET_PROJECT_USAGE, usage);

                return usage;
            },
            [PROJECT_USAGE_ACTIONS.FETCH_CURRENT_ROLLUP]: async function({commit, rootGetters}: any): Promise<ProjectUsage> {
                const projectID: string = rootGetters.selectedProject.id;

                const endDate = new Date();
                const startDate = new Date(Date.UTC(endDate.getUTCFullYear(), endDate.getUTCMonth(), 1));
                const dateRange = new DateRange(startDate, endDate);

                const usage: ProjectUsage = await api.get(projectID, dateRange.startDate, dateRange.endDate);

                commit(PROJECT_USAGE_MUTATIONS.SET_DATE, dateRange);
                commit(PROJECT_USAGE_MUTATIONS.SET_PROJECT_USAGE, usage);

                return usage;
            },
            [PROJECT_USAGE_ACTIONS.FETCH_PREVIOUS_ROLLUP]: async function({commit, rootGetters}: any): Promise<ProjectUsage> {
                const projectID = rootGetters.selectedProject.id;

                const date = new Date();
                const startDate = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() - 1, 1));
                const endDate = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), 0, 23, 59, 59));
                const dateRange = new DateRange(startDate, endDate);

                const usage: ProjectUsage = await api.get(projectID, dateRange.startDate, dateRange.endDate);

                commit(PROJECT_USAGE_MUTATIONS.SET_DATE, dateRange);
                commit(PROJECT_USAGE_MUTATIONS.SET_PROJECT_USAGE, usage);

                return usage;
            },
            [PROJECT_USAGE_ACTIONS.CLEAR]: function({commit}): void {
                commit(PROJECT_USAGE_MUTATIONS.CLEAR);
            },
        },
    };
}
