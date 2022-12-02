// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../../mock/api/projects';

import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { Project, ProjectLimits } from '@/types/projects';

import ProjectUsage from '@/components/project/usage/ProjectUsage.vue';

const localVue = createLocalVue();
localVue.use(Vuex);

const projectLimits = new ProjectLimits(1000, 100, 1000, 100);
const projectsApi = new ProjectsApiMock();
projectsApi.setMockLimits(projectLimits);
const projectsModule = makeProjectsModule(projectsApi);

const store = new Vuex.Store({ modules: { projectsModule } });
const project = new Project('id', 'test', 'test', 'test', 'ownedId', false);

describe('ProjectUsage.vue', () => {
    it('renders correctly', (): void => {
        store.commit(PROJECTS_MUTATIONS.ADD, project);
        store.commit(PROJECTS_MUTATIONS.SELECT_PROJECT, project.id);
        store.commit(PROJECTS_MUTATIONS.SET_LIMITS, projectLimits);

        const wrapper = shallowMount(ProjectUsage, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
