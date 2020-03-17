// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import NewProjectPopup from '@/components/project/NewProjectPopup.vue';

import { appStateModule } from '@/store/modules/appState';
import { makeProjectsModule } from '@/store/modules/projects';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { NotificatorPlugin } from '@/utils/plugins/notificator';
import { createLocalVue, mount, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../mock/api/projects';

const notificationPlugin = new NotificatorPlugin();
const localVue = createLocalVue();
localVue.use(Vuex);
localVue.use(notificationPlugin);

const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const store = new Vuex.Store({ modules: { appStateModule, projectsModule }});

describe('NewProjectPopup.vue', () => {
    it('renders correctly', async (): Promise<void> => {
        await store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_PROJ);

        const wrapper = mount(NewProjectPopup, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();

        await wrapper.vm.setProjectName('testName');
        await wrapper.vm.createProjectClick();

        expect(wrapper).toMatchSnapshot();
    });

    it('closes correctly', async (): Promise<void> => {
        await store.dispatch(APP_STATE_ACTIONS.TOGGLE_NEW_PROJ);

        const wrapper = shallowMount(NewProjectPopup, {
            store,
            localVue,
        });

        await wrapper.find('.new-project-popup__close-cross-container').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });
});
