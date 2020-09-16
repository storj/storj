// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import ProjectDropdown from '@/components/header/projectsDropdown/ProjectDropdown.vue';

import { appStateModule } from '@/store/modules/appState';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { Project } from '@/types/projects';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../mock/api/projects';

const localVue = createLocalVue();
localVue.use(Vuex);

const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);
const project1 = new Project('testId1', 'testName1', '');
const project2 = new Project('testId2', 'testName2', '');

const store = new Vuex.Store({ modules: { projectsModule, appStateModule }});

describe('ProjectDropdown', () => {
    it('renders correctly', () => {
        store.commit(PROJECTS_MUTATIONS.SET_PROJECTS, [project1, project2]);
        store.commit(PROJECTS_MUTATIONS.SELECT_PROJECT, project1.id);

        const wrapper = shallowMount(ProjectDropdown, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
