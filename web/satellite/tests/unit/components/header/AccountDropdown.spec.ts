// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';
import Vuex from 'vuex';

import AccountDropdown from '@/components/header/AccountDropdown.vue';

import { RouteConfig, router } from '@/router';
import { appStateModule } from '@/store/modules/appState';
import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

const store = new Vuex.Store({ modules: { appStateModule }});

describe('AccountDropdown', () => {
    it('renders correctly', (): void => {
        const wrapper = mount(AccountDropdown, {
            store,
            localVue,
            router,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if on onBoarding tour', async (): Promise<void> => {
        const wrapper = mount(AccountDropdown, {
            store,
            localVue,
            router,
        });

        await router.push(RouteConfig.OnboardingTour.path);

        expect(wrapper).toMatchSnapshot();
    });

    it('router works correctly', async (): Promise<void> => {
        const routerSpy = sinon.spy();
        const wrapper = mount(AccountDropdown, {
            store,
            localVue,
            router,
        });

        wrapper.vm.onAccountSettingsClick = routerSpy;
        wrapper.vm.onLogoutClick = routerSpy;

        await router.push(RouteConfig.ProjectDashboard.path);
        await wrapper.find('.settings').trigger('click');

        expect(routerSpy.callCount).toBe(1);

        await wrapper.find('.logout').trigger('click');

        expect(routerSpy.callCount).toBe(2);
    });
});
