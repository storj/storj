// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import ProjectCreationSuccessPopup from '@/components/project/ProjectCreationSuccessPopup.vue';

import { appStateModule } from '@/store/modules/appState';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { mount } from '@vue/test-utils';

const store = new Vuex.Store({ modules: { appStateModule }});

describe('ProjectCreationSuccessPopup.vue', () => {
    it('renders correctly', async (): Promise<void> => {
        await store.dispatch(APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_PROJECT_CREATION_POPUP);

        const wrapper = mount(ProjectCreationSuccessPopup, {
            store,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('closes correctly', async (): Promise<void> => {
        const wrapper = mount(ProjectCreationSuccessPopup, {
            store,
        });

        await wrapper.find('.project-creation-success-popup__close-cross-container').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });
});
