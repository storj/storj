// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import CreateProject from '@/components/project/CreateProject.vue';

import { makeProjectsModule } from '@/store/modules/projects';
import { NotificatorPlugin } from '@/utils/plugins/notificator';
import { createLocalVue, mount } from '@vue/test-utils';

import { ProjectsApiMock } from '../mock/api/projects';

const notificationPlugin = new NotificatorPlugin();
const localVue = createLocalVue();
localVue.use(Vuex);
localVue.use(notificationPlugin);

const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const store = new Vuex.Store({ modules: { projectsModule }});

describe('CreateProject.vue', (): void => {
    it('renders correctly', (): void => {
        const wrapper = mount(CreateProject, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with project name', async (): Promise<void> => {
        const wrapper = mount(CreateProject, {
            store,
            localVue,
        });

        await wrapper.vm.setProjectName('testName');

        expect(wrapper.findAll('.disabled').length).toBe(0);
    });
});
