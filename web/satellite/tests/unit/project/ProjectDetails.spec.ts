// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import ProjectDetails from '@/components/project/ProjectDetails.vue';

import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { Project } from '@/types/projects';
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
const project = new Project('id', 'test', 'test', 'test', 'ownedId', false);

describe('ProjectDetails.vue', () => {
    it('renders correctly', (): void => {
        store.commit(PROJECTS_MUTATIONS.ADD, project);
        store.commit(PROJECTS_MUTATIONS.SELECT_PROJECT, project.id);

        const wrapper = mount(ProjectDetails, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('editing works correctly', async (): Promise<void> => {
        const wrapper = mount(ProjectDetails, {
            store,
            localVue,
        });

        await wrapper.vm.toggleEditing();

        expect(wrapper).toMatchSnapshot();

        wrapper.vm.$data.newDescription = 'new description';
        await wrapper.vm.onSaveButtonClick();

        expect(wrapper).toMatchSnapshot();
        await expect(wrapper.vm.storedDescription).toMatch('new description');
    });
});
