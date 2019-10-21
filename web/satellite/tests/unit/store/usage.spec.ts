// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import { ProjectsApiGql } from '@/api/projects';
import { ProjectUsageApiGql } from '@/api/usage';
import { makeProjectsModule } from '@/store/modules/projects';
import { makeUsageModule, PROJECT_USAGE_ACTIONS, PROJECT_USAGE_MUTATIONS } from '@/store/modules/usage';
import { Project } from '@/types/projects';
import { DateRange, ProjectUsage } from '@/types/usage';
import { createLocalVue } from '@vue/test-utils';

const Vue = createLocalVue();
const projectUsageApi = new ProjectUsageApiGql();
const usageModule = makeUsageModule(projectUsageApi);

const projectsApi = new ProjectsApiGql();
const projectsModule = makeProjectsModule(projectsApi);
const selectedProject = new Project('', '', '', '');
selectedProject.id = '1';
projectsModule.state.selectedProject = selectedProject;

const testDate1 = new Date();
testDate1.setDate(1);
const testDate2 = new Date();
testDate2.setDate(2);
const testUsage = new ProjectUsage(2, 3, 4, testDate1, testDate2);
const now = new Date();

Vue.use(Vuex);

const store = new Vuex.Store({ modules: { usageModule, projectsModule } });

const state = (store.state as any).usageModule;

describe('mutations', () => {
    beforeEach(() => {
        createLocalVue().use(Vuex);
    });

    it('fetch project usage', () => {
        store.commit(PROJECT_USAGE_MUTATIONS.SET_PROJECT_USAGE, testUsage);

        expect(state.projectUsage.storage).toBe(2);
        expect(state.projectUsage.egress).toBe(3);
        expect(state.projectUsage.objectCount).toBe(4);
        expect(state.startDate.toDateString()).toBe(now.toDateString());
        expect(state.endDate.toDateString()).toBe(now.toDateString());
    });

    it('set dates', () => {
        const dateRange: DateRange = new DateRange(testDate1, testDate2);
        store.commit(PROJECT_USAGE_MUTATIONS.SET_DATE, dateRange);

        expect(state.startDate.toDateString()).toBe(testDate1.toDateString());
        expect(state.endDate.toDateString()).toBe(testDate2.toDateString());
    });

    it('clear usage', () => {
        store.commit(PROJECT_USAGE_MUTATIONS.CLEAR);

        expect(state.projectUsage.storage).toBe(0);
        expect(state.projectUsage.egress).toBe(0);
        expect(state.projectUsage.objectCount).toBe(0);
        expect(state.startDate.toDateString()).toBe(now.toDateString());
        expect(state.endDate.toDateString()).toBe(now.toDateString());
    });
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
        createLocalVue().use(Vuex);
    });

    it('success fetch project usage', async () => {
        jest.spyOn(projectUsageApi, 'get').mockReturnValue(
            Promise.resolve(testUsage),
        );
        const dateRange: DateRange = new DateRange(testDate1, testDate2);

        await store.dispatch(PROJECT_USAGE_ACTIONS.FETCH, dateRange);

        expect(state.projectUsage.storage).toBe(2);
        expect(state.projectUsage.egress).toBe(3);
        expect(state.projectUsage.objectCount).toBe(4);
        expect(state.startDate.toDateString()).toBe(testDate1.toDateString());
        expect(state.endDate.toDateString()).toBe(testDate2.toDateString());
    });

    it('success fetch current project usage', async () => {
        jest.spyOn(projectUsageApi, 'get').mockReturnValue(
            Promise.resolve(testUsage),
        );

        const firstDate = new Date();
        firstDate.setDate(1);

        await store.dispatch(PROJECT_USAGE_ACTIONS.FETCH_CURRENT_ROLLUP);

        expect(state.projectUsage.storage).toBe(2);
        expect(state.projectUsage.egress).toBe(3);
        expect(state.projectUsage.objectCount).toBe(4);
        expect(state.startDate.toDateString()).toBe(firstDate.toDateString());
        expect(state.endDate.toDateString()).toBe(now.toDateString());
    });

    it('success fetch previous project usage', async () => {
        jest.spyOn(projectUsageApi, 'get').mockReturnValue(
            Promise.resolve(testUsage),
        );

        const date = new Date();

        const firstDate = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth() - 1, 1));
        const secondDate = new Date(Date.UTC(date.getUTCFullYear(), date.getUTCMonth(), 0, 23, 59, 59));

        await store.dispatch(PROJECT_USAGE_ACTIONS.FETCH_PREVIOUS_ROLLUP);

        expect(state.projectUsage.storage).toBe(2);
        expect(state.projectUsage.egress).toBe(3);
        expect(state.projectUsage.objectCount).toBe(4);
        expect(state.startDate.toDateString()).toBe(firstDate.toDateString());
        expect(state.endDate.toDateString()).toBe(secondDate.toDateString());
    });

    it('success clear usage', async () => {
        await store.dispatch(PROJECT_USAGE_ACTIONS.CLEAR);

        expect(state.projectUsage.storage).toBe(0);
        expect(state.projectUsage.egress).toBe(0);
        expect(state.projectUsage.objectCount).toBe(0);
        expect(state.startDate.toDateString()).toBe(now.toDateString());
        expect(state.endDate.toDateString()).toBe(now.toDateString());
    });

    it('create throws an error when create api call fails', async () => {
        state.projects = [];
        jest.spyOn(projectUsageApi, 'get').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(PROJECT_USAGE_ACTIONS.FETCH_PREVIOUS_ROLLUP);
            expect(true).toBe(false);
        } catch (error) {
            expect(state.projectUsage.storage).toBe(0);
            expect(state.projectUsage.egress).toBe(0);
            expect(state.projectUsage.objectCount).toBe(0);
            expect(state.startDate.toDateString()).toBe(now.toDateString());
            expect(state.endDate.toDateString()).toBe(now.toDateString());
        }
    });
});
