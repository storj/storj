// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import router, { RouteConfig } from '@/router';
import { makeApiKeysModule } from '@/store/modules/apiKeys';
import { appStateModule } from '@/store/modules/appState';
import { makeBucketsModule } from '@/store/modules/buckets';
import { makeNotificationsModule } from '@/store/modules/notifications';
import { makeProjectMembersModule } from '@/store/modules/projectMembers';
import { makeProjectsModule } from '@/store/modules/projects';
import { makeUsageModule } from '@/store/modules/usage';
import { makeUsersModule } from '@/store/modules/users';
import { User } from '@/types/users';
import { AuthToken } from '@/utils/authToken';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { AppState } from '@/utils/constants/appStateEnum';
import Dashboard from '@/views/Dashboard.vue';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { ApiKeysMock } from '../mock/api/apiKeys';
import { BucketsMock } from '../mock/api/buckets';
import { ProjectMembersApiMock } from '../mock/api/projectMembers';
import { ProjectsApiMock } from '../mock/api/projects';
import { ProjectUsageMock } from '../mock/api/usage';
import { UsersApiMock } from '../mock/api/users';

const localVue = createLocalVue();
localVue.use(Vuex);

const usersApi = new UsersApiMock();
const projectsApi = new ProjectsApiMock();

usersApi.setMockUser(new User('1', '2', '3', '4', '5'));
projectsApi.setMockProjects([]);

const usersModule = makeUsersModule(usersApi);
const projectsModule = makeProjectsModule(projectsApi);
const apiKeysModule = makeApiKeysModule(new ApiKeysMock());
const teamMembersModule = makeProjectMembersModule(new ProjectMembersApiMock());
const bucketsModule = makeBucketsModule(new BucketsMock());
const usageModule = makeUsageModule(new ProjectUsageMock());
const notificationsModule = makeNotificationsModule();

const store = new Vuex.Store({
    modules: {
        notificationsModule,
        usageModule,
        bucketsModule,
        apiKeysModule,
        usersModule,
        projectsModule,
        appStateModule,
        teamMembersModule,
    },
});

describe('Dashboard', () => {
    beforeEach(() => {
        jest.resetAllMocks();
    });

    it('renders correctly when data is loading', () => {
        const wrapper = shallowMount(Dashboard, {
            store,
            localVue,
            router,
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.findAll('.loading-overlay.active').length).toBe(1);
        expect(wrapper.findAll('.dashboard-container__wrap').length).toBe(0);
    });

    it('renders correctly when data is loaded', () => {
        store.dispatch(APP_STATE_ACTIONS.CHANGE_STATE, AppState.LOADED);

        const wrapper = shallowMount(Dashboard, {
            store,
            localVue,
            router,
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.findAll('.loading-overlay active').length).toBe(0);
        expect(wrapper.findAll('.dashboard-container__wrap').length).toBe(1);
    });

    it('loads routes correctly when authorithed without project with available routes', async () => {
        jest.spyOn(AuthToken, 'get').mockReturnValue('authToken');

        const availableWithoutProject = [
            RouteConfig.Account.with(RouteConfig.Billing).path,
            RouteConfig.Account.with(RouteConfig.Profile).path,
            RouteConfig.Account.with(RouteConfig.PaymentMethods).path,
        ];

        for (let i = 0; i < availableWithoutProject.length; i++) {
            const wrapper = await shallowMount(Dashboard, {
                localVue,
                router,
                store,
            });

            setTimeout(() => {
                expect(wrapper.vm.$router.currentRoute.path).toBe(availableWithoutProject[i]);
            }, 50);
        }
    });

    it('loads routes correctly when authorithed without project with unavailable routes', async () => {
        jest.spyOn(AuthToken, 'get').mockReturnValue('authToken');

        const unavailableWithoutProject = [
            RouteConfig.ApiKeys.path,
            RouteConfig.Buckets.path,
            RouteConfig.Team.path,
            RouteConfig.ProjectOverview.with(RouteConfig.UsageReport).path,
        ];

        for (let i = 0; i < unavailableWithoutProject.length; i++) {
            await router.push(unavailableWithoutProject[i]);

            const wrapper = await shallowMount(Dashboard, {
                localVue,
                router,
                store,
            });

            setTimeout(() => {
                expect(wrapper.vm.$router.currentRoute.path).toBe(RouteConfig.ProjectOverview.with(RouteConfig.ProjectDetails).path);
            }, 50);
        }

    });

    it('loads routes correctly when not authorithed', () => {
        const wrapper = shallowMount(Dashboard, {
            store,
            localVue,
            router,
        });

        setTimeout(() => {
            expect(wrapper.vm.$router.currentRoute.path).toBe(RouteConfig.Login.path);
        }, 50);
    });
});
