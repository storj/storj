// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import { appStateModule } from '@/store/modules/appState';
import { makeNotificationsModule } from '@/store/modules/notifications';
import { makeProjectMembersModule } from '@/store/modules/projectMembers';
import { ProjectMember, ProjectMemberHeaderState, ProjectMembersPage } from '@/types/projectMembers';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { AppState } from '@/utils/constants/appStateEnum';
import Dashboard from '@/views/Dashboard.vue';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { ProjectMembersApiMock } from '../mock/api/projectMembers';

const localVue = createLocalVue();

localVue.use(Vuex);

const store = new Vuex.Store({ modules: { appStateModule } });

describe('Dashboard', () => {
    it('renders correctly when data is loading', () => {
        const wrapper = shallowMount(Dashboard, {
            store,
            localVue,
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
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.findAll('.loading-overlay active').length).toBe(0);
        expect(wrapper.findAll('.dashboard-container__wrap').length).toBe(1);
    });
});
