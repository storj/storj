// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { AccessGrantsMock } from '../../mock/api/accessGrants';
import { BucketsMock } from '../../mock/api/buckets';

import { RouteConfig, router } from '@/router';
import { makeAccessGrantsModule } from '@/store/modules/accessGrants';
import { makeBucketsModule } from '@/store/modules/buckets';
import { makeNotificationsModule } from '@/store/modules/notifications';
import { FetchState } from '@/utils/constants/fetchStateEnum';
import { NotificatorPlugin } from '@/utils/plugins/notificator';
import { AnalyticsHttpApi } from '@/api/analytics';
import DashboardArea from '@/views/DashboardArea.vue';

const localVue = createLocalVue();
localVue.use(Vuex);

const accessGrantsModule = makeAccessGrantsModule(new AccessGrantsMock());
const bucketsModule = makeBucketsModule(new BucketsMock());
const notificationsModule = makeNotificationsModule();

const store = new Vuex.Store({
    modules: {
        notificationsModule,
        bucketsModule,
        accessGrantsModule,
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
