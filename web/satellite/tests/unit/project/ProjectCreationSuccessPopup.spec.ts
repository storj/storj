// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import ProjectCreationSuccessPopup from '@/components/project/ProjectCreationSuccessPopup.vue';

import { appStateModule } from '@/store/modules/appState';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { createLocalVue, mount, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

const store = new Vuex.Store({ modules: { appStateModule }});

describe('ProjectCreationSuccessPopup.vue', () => {
    it('renders correctly', async (): Promise<void> => {
        await store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_PROJECT_CREATION_POPUP);

        const wrapper = shallowMount(ProjectCreationSuccessPopup, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('closes correctly', async (): Promise<void> => {
        const wrapper = shallowMount(ProjectCreationSuccessPopup, {
            store,
            localVue,
        });

        await wrapper.find('.project-creation-success-popup__close-cross-container').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });
});
