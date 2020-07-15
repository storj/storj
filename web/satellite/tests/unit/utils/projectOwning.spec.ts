// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import { makeProjectsModule, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { makeUsersModule, USER_MUTATIONS } from '@/store/modules/users';
import { Project } from '@/types/projects';
import { User } from '@/types/users';
import { ProjectOwning } from '@/utils/projectOwning';
import { createLocalVue } from '@vue/test-utils';

import { ProjectsApiMock } from '../mock/api/projects';
import { UsersApiMock } from '../mock/api/users';

const usersApi = new UsersApiMock();
const usersModule = makeUsersModule(usersApi);
const projectsApi = new ProjectsApiMock();
const projectsModule = makeProjectsModule(projectsApi);

const localVue = createLocalVue();
localVue.use(Vuex);

const store = new Vuex.Store({ modules: { usersModule, projectsModule }});

describe('projectOwning', () => {
    it('user has no project', () => {
        const user = new User('ownerId');
        store.commit(USER_MUTATIONS.SET_USER, user);

        expect(new ProjectOwning(store).usersProjectsCount()).toBe(0);
    });

    it('user has project', () => {
        const project = new Project('id', 'test', 'test', 'test', 'ownerId', true);
        store.commit(PROJECTS_MUTATIONS.ADD, project);

        expect(new ProjectOwning(store).usersProjectsCount()).toBe(1);
    });
});
