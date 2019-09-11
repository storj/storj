// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import { ProjectsApiGql } from '@/api/projects';
import { makeProjectsModule, PROJECTS_ACTIONS, PROJECTS_MUTATIONS } from '@/store/modules/projects';
import { Project } from '@/types/projects';
import { createLocalVue } from '@vue/test-utils';

const Vue = createLocalVue();
const projectsApi = new ProjectsApiGql();
const { FETCH, CREATE, SELECT, DELETE, CLEAR, UPDATE } = PROJECTS_ACTIONS;
const { ADD, SET_PROJECTS, SELECT_PROJECT, UPDATE_PROJECT, REMOVE, CLEAR_PROJECTS } = PROJECTS_MUTATIONS;

const projectsModule = makeProjectsModule(projectsApi);
const selectedProject = new Project('1', '', '', '');
projectsModule.state.selectedProject = selectedProject;

Vue.use(Vuex);

const store = new Vuex.Store({ modules: { projectsModule } });

const state = (store.state as any).projectsModule;

const projects = [
    new Project('11', 'name', 'descr', '23'),
    new Project('1', 'name2', 'descr2', '24'),
];

const project = new Project('11', 'name', 'descr', '23');

describe('mutations', () => {
    beforeEach(() => {
        createLocalVue().use(Vuex);
    });

    it('add project', () => {

        store.commit(ADD, project);

        expect(state.projects[0].id).toBe(project.id);
        expect(state.projects[0].name).toBe(project.name);
        expect(state.projects[0].description).toBe(project.description);
        expect(state.projects[0].createdAt).toBe(project.createdAt );
    });

    it('set projects', () => {
        store.commit(SET_PROJECTS, projects);

        expect(state.projects).toBe(projects);
        expect(state.selectedProject.id).toBe('1');
    });

    it('select project', () => {
        state.projects = projects;

        store.commit(SELECT_PROJECT, '11');
        expect(state.selectedProject.id).toBe('11');
    });

    it('update project', () => {
        state.projects = projects;

        const newDescription = 'newDescription';

        store.commit(UPDATE_PROJECT, { id: '11', description: newDescription });

        expect(state.projects.find((pr: Project) => pr.id === '11').description).toBe(newDescription);
    });

    it('remove project', () => {
        state.projects = projects;

        store.commit(REMOVE, '11');

        expect(state.projects.length).toBe(1);
        expect(state.projects[0].id).toBe('1');
    });

    it('clear projects', () => {
        state.projects = projects;

        store.commit(CLEAR_PROJECTS);

        expect(state.projects.length).toBe(0);
    });
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
        createLocalVue().use(Vuex);
    });

    it('success fetch projects', async () => {
        jest.spyOn(projectsApi, 'get').mockReturnValue(
            Promise.resolve(projects)
        );

        await store.dispatch(FETCH);

        expect(state.projects).toBe(projects);
    });

    it('fetch throws an error when api call fails', async () => {
        state.projects = [];
        jest.spyOn(projectsApi, 'get').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(FETCH);
        } catch (error) {
            expect(state.projects.length).toBe(0);
        }
    });

    it('success create project', async () => {
        state.projects = [];
        jest.spyOn(projectsApi, 'create').mockReturnValue(
            Promise.resolve(project)
        );

        await store.dispatch(CREATE, {name:'', description: ''});
        expect(state.projects.length).toBe(1);
    });

    it('create throws an error when create api call fails', async () => {
        state.projects = [];
        jest.spyOn(projectsApi, 'create').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(CREATE, 'testName');
            expect(true).toBe(false);
        } catch (error) {
            expect(state.projects.length).toBe(0);
        }
    });

    it('success delete apiKeys', async () => {
        jest.spyOn(projectsApi, 'delete').mockReturnValue(
            Promise.resolve()
        );

        state.projects = projects;

        await store.dispatch(DELETE, '11');

        expect(state.projects.length).toBe(1);
        expect(state.projects[0].id).toBe('1');
    });

    it('delete throws an error when api call fails', async () => {
        jest.spyOn(projectsApi, 'delete').mockImplementation(() => { throw new Error(); });

        state.projects = projects;

        try {
            await store.dispatch(DELETE, '11');
            expect(true).toBe(false);
        } catch (error) {
            expect(state.projects).toEqual(projects);
        }
    });

    it('success select project', () => {
        state.projects = projects;

        store.dispatch(SELECT, '1');

        expect(state.selectedProject.id).toEqual('1');
    });

    it('success update project', async () => {
        jest.spyOn(projectsApi, 'update').mockReturnValue(
            Promise.resolve()
        );

        state.projects = projects;
        const newDescription = 'newDescription1';

        await store.dispatch(UPDATE, { id: '1', description: newDescription });

        expect(state.projects.find((pr: Project) => pr.id === '1').description).toBe(newDescription);
    });

    it('update throws an error when api call fails', async () => {
        jest.spyOn(projectsApi, 'update').mockImplementation(() => { throw new Error(); });

        state.projects = projects;
        const newDescription = 'newDescription2';

        try {
            await store.dispatch(UPDATE, { id: '1', description: newDescription });
            expect(true).toBe(false);
        } catch (error) {
            expect(state.projects.find((pr: Project) => pr.id === '1').description).toBe('newDescription1');
        }
    });

    it('success clearProjects', () => {
        state.projects = projects;
        store.dispatch(CLEAR);

        expect(state.projects.length).toEqual(0);
    });
});

describe('getters', () => {
    beforeEach(() => {
        createLocalVue().use(Vuex);
    });

    it('selectedProject', () => {
        store.commit(PROJECTS_MUTATIONS.SET_PROJECTS, projects);
        store.commit(PROJECTS_MUTATIONS.SELECT_PROJECT, '1');

        const selectedProject = store.getters.selectedProject;

        expect(selectedProject.id).toBe('1');
    });

    it('apiKeys array', () => {
        store.commit(PROJECTS_MUTATIONS.SET_PROJECTS, projects);

        const allProjects = store.getters.projects;

        expect(allProjects.length).toEqual(2);
    });
});
