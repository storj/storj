// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import ProjectDashboard from '@/components/project/ProjectDashboard.vue';

import { appStateModule } from '@/store/modules/appState';
import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { makeUsersModule } from '@/store/modules/users';
import { Project } from '@/types/projects';
import { SegmentioPlugin } from '@/utils/plugins/segment';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { ProjectsApiMock } from '../mock/api/projects';
import { UsersApiMock } from '../mock/api/users';

const segmentioPlugin = new SegmentioPlugin();
const localVue = createLocalVue();
localVue.use(Vuex);
localVue.use(segmentioPlugin);

const usersApi = new UsersApiMock();
const usersModule = makeUsersModule(usersApi);
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);

const store = new Vuex.Store({ modules: { appStateModule, usersModule, projectsModule }});
const project = new Project('id', 'test', 'test', 'test', 'ownedId', false);

describe('ProjectDashboard.vue', () => {
    it('renders correctly', (): void => {
        store.commit(PROJECTS_MUTATIONS.ADD, project);
        store.commit(PROJECTS_MUTATIONS.SELECT_PROJECT, project.id);

        const wrapper = shallowMount(ProjectDashboard, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
