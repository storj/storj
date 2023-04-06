// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { AccessGrantsMock } from '../../mock/api/accessGrants';
import { BucketsMock } from '../../mock/api/buckets';
import { PaymentsMock } from '../../mock/api/payments';
import { ProjectMembersApiMock } from '../../mock/api/projectMembers';
import { ProjectsApiMock } from '../../mock/api/projects';
import { UsersApiMock } from '../../mock/api/users';
import { FrontendConfigApiMock } from '../../mock/api/config';

import { RouteConfig, router } from '@/router';
import { makeAccessGrantsModule } from '@/store/modules/accessGrants';
import { makeAppStateModule } from '@/store/modules/appState';
import { makeBucketsModule } from '@/store/modules/buckets';
import { makeNotificationsModule } from '@/store/modules/notifications';
import { makePaymentsModule } from '@/store/modules/payments';
import { makeProjectMembersModule } from '@/store/modules/projectMembers';
import { makeProjectsModule } from '@/store/modules/projects';
import { User } from '@/types/users';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { FetchState } from '@/utils/constants/fetchStateEnum';
import { NotificatorPlugin } from '@/utils/plugins/notificator';
import DashboardArea from '@/views/DashboardArea.vue';
import { AnalyticsHttpApi } from '@/api/analytics';

const localVue = createLocalVue();
localVue.use(Vuex);

const usersApi = new UsersApiMock();
const projectsApi = new ProjectsApiMock();

usersApi.setMockUser(new User('1', '2', '3', '4', '5', '6', 1));
projectsApi.setMockProjects([]);

const appStateModule = makeAppStateModule(new FrontendConfigApiMock());
const projectsModule = makeProjectsModule(projectsApi);
const accessGrantsModule = makeAccessGrantsModule(new AccessGrantsMock());
const teamMembersModule = makeProjectMembersModule(new ProjectMembersApiMock());
const bucketsModule = makeBucketsModule(new BucketsMock());
const notificationsModule = makeNotificationsModule();
const paymentsModule = makePaymentsModule(new PaymentsMock());

const store = new Vuex.Store({
    modules: {
        notificationsModule,
        bucketsModule,
        accessGrantsModule,
        projectsModule,
        appStateModule,
        teamMembersModule,
        paymentsModule,
    },
});

localVue.use(new NotificatorPlugin(store));

describe('Dashboard', () => {
    beforeEach(() => {
        jest.resetAllMocks();
        jest.spyOn(AnalyticsHttpApi.prototype, 'errorEventTriggered').mockImplementation(() => Promise.resolve());
    });

    it('renders correctly when data is loading', () => {
        const wrapper = shallowMount(DashboardArea, {
            store,
            localVue,
            router,
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.findAll('.loading-overlay.active').length).toBe(1);
        expect(wrapper.findAll('.dashboard-container__wrap').length).toBe(0);
    });

    it('renders correctly when data is loaded', () => {
        store.dispatch(APP_STATE_ACTIONS.CHANGE_FETCH_STATE, FetchState.LOADED);

        const wrapper = shallowMount(DashboardArea, {
            store,
            localVue,
            router,
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.findAll('.loading-overlay active').length).toBe(0);
        expect(wrapper.findAll('.dashboard__wrap').length).toBe(1);
    });

    it('loads routes correctly when authorithed without project with available routes', async () => {
        const availableWithoutProject = [
            RouteConfig.Account.with(RouteConfig.Billing).path,
            RouteConfig.Account.with(RouteConfig.Settings).path,
        ];

        for (let i = 0; i < availableWithoutProject.length; i++) {
            const wrapper = await shallowMount(DashboardArea, {
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
        const unavailableWithoutProject = [
            RouteConfig.AccessGrants.path,
            RouteConfig.Users.path,
            RouteConfig.ProjectDashboard.path,
        ];

        for (let i = 0; i < unavailableWithoutProject.length; i++) {
            await router.push(unavailableWithoutProject[i]);

            const wrapper = await shallowMount(DashboardArea, {
                localVue,
                router,
                store,
            });

            setTimeout(() => {
                expect(wrapper.vm.$router.currentRoute.path).toBe(RouteConfig.ProjectDashboard.path);
            }, 50);
        }

    });

    it('loads routes correctly when not authorithed', () => {
        const wrapper = shallowMount(DashboardArea, {
            store,
            localVue,
            router,
        });

        setTimeout(() => {
            expect(wrapper.vm.$router.currentRoute.path).toBe(RouteConfig.Login.path);
        }, 50);
    });
});
