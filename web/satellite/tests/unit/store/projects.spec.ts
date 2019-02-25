// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { projectsModule } from '@/store/modules/projects';
import * as api from '@/api/projects';
import { createProjectRequest, deleteProjectRequest, fetchProjectsRequest, updateProjectRequest } from '@/api/projects';
import { PROJECTS_MUTATIONS } from '@/store/mutationConstants';
import { createLocalVue } from '@vue/test-utils';
import Vuex from 'vuex';

const mutations = projectsModule.mutations;

describe('mutations', () => {
    beforeEach(() => {
        createLocalVue().use(Vuex);
    });
    it('create project', () => {
        const state = {
            projects: [],
        };
        const project = {
            name: 'testName',
        };
        const store = new Vuex.Store({state, mutations});

        store.commit(PROJECTS_MUTATIONS.CREATE, project);

        expect(state.projects.length).toBe(1);

        const mutatedProject: Project = state.projects[0];

        expect(mutatedProject.name).toBe('testName');
    });

    it('fetch project', () => {
        const state = {
            projects: []
        };

        const store = new Vuex.Store({state, mutations});

        const projectsToPush = [{id: '1'}, {id: '2'}];

        store.commit(PROJECTS_MUTATIONS.FETCH, projectsToPush);

        expect(state.projects.length).toBe(2);
    });

    it('success select project', () => {
        const state = {
            projects: [{id: '1'}, {id: 'testId'}, {id: '2'}, ],
            selectedProject: {
                id: ''
            }
        };

        const store = new Vuex.Store({state, mutations});

        const projectId = 'testId';

        store.commit(PROJECTS_MUTATIONS.SELECT, projectId);

        expect(state.selectedProject.id).toBe('testId');
    });

    it('error select project', () => {
        const state = {
            projects: [{id: '1'}, {id: 'testId'}, {id: '2'}, ],
            selectedProject: {
                id: 'old'
            }
        };

        const store = new Vuex.Store({state, mutations});

        const projectId = '3';

        store.commit(PROJECTS_MUTATIONS.SELECT, projectId);

        expect(state.selectedProject.id).toBe('old');
    });

    it('error update project not exist', () => {
        const state = {
            projects: [{id: '1'}, {id: 'testId'}, {id: '2'}, ],
            selectedProject: {
                id: 'old'
            }
        };

        const store = new Vuex.Store({state, mutations});

        const projectId = {id: '3'};

        store.commit(PROJECTS_MUTATIONS.UPDATE, projectId);

        expect(state.selectedProject.id).toBe('old');
    });

    it('error update project not selected', () => {
        const state = {
            projects: [{id: '1'}, {id: 'testId'}, {id: '2'}, ],
            selectedProject: {
                id: 'old',
                description: 'oldD'
            }
        };

        const store = new Vuex.Store({state, mutations});

        const project = {id: '2', description: 'newD'};

        store.commit(PROJECTS_MUTATIONS.UPDATE, project);

        expect(state.selectedProject.id).toBe('old');
        expect(state.selectedProject.description).toBe('oldD');
    });

    it('success update project', () => {
        const state = {
            projects: [{id: '1'}, {id: 'testId'}, {id: '2'}],
            selectedProject: {
                id: '2',
                description: 'oldD'
            }
        };

        const store = new Vuex.Store({state, mutations});

        const project = {id: '2', description: 'newD'};

        store.commit(PROJECTS_MUTATIONS.UPDATE, project);

        expect(state.selectedProject.id).toBe('2');
        expect(state.selectedProject.description).toBe('newD');
    });

    it('error delete project', () => {
        const state = {
            selectedProject: {
                id: '1',
            }
        };

        const store = new Vuex.Store({state, mutations});

        const projectId = '2';

        store.commit(PROJECTS_MUTATIONS.DELETE, projectId);

        expect(state.selectedProject.id).toBe('1');
    });

    it('success delete project', () => {
        const state = {
            selectedProject: {
                id: '2',
            }
        };

        const store = new Vuex.Store({state, mutations});

        const projectId = '2';

        store.commit(PROJECTS_MUTATIONS.DELETE, projectId);

        expect(state.selectedProject.id).toBe('');
    });
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
    });
    it('success fetch project', async () => {
        jest.spyOn(api, 'fetchProjectsRequest').mockReturnValue(
            Promise.resolve(<RequestResponse<Project[]>>{isSuccess: true, data: [{id: '1'}, {id: '2'}]})
        );

        const commit = jest.fn();

        const dispatchResponse = await projectsModule.actions.fetchProjects({commit});

        expect(dispatchResponse.isSuccess).toBeTruthy();
        expect(commit).toHaveBeenCalledWith(PROJECTS_MUTATIONS.FETCH, [{id: '1'}, {id: '2'}]);
    });

    it('error fetch project', async () => {
        jest.spyOn(api, 'fetchProjectsRequest').mockReturnValue(
            Promise.resolve(<RequestResponse<Project[]>>{isSuccess: false})
        );
        const commit = jest.fn();
        const dispatchResponse = await projectsModule.actions.fetchProjects({commit});

        expect(dispatchResponse.isSuccess).toBeFalsy();
        expect(commit).toHaveBeenCalledTimes(0);
    });

    it('success create project', async () => {
        jest.spyOn(api, 'createProjectRequest').mockReturnValue(
            Promise.resolve(<RequestResponse<Project>>{isSuccess: true, data: {id: '1'}})
        );
        const commit = jest.fn();
        const project: Project = {
            name: '',
            id: '',
            description: '',
            isSelected: false,
            createdAt: ''
        };

        const dispatchResponse = await projectsModule.actions.createProject({commit}, project);

        expect(dispatchResponse.isSuccess).toBeTruthy();
        expect(commit).toHaveBeenCalledWith(PROJECTS_MUTATIONS.CREATE, {id: '1'});
    });

    it('error create project', async () => {
        jest.spyOn(api, 'createProjectRequest').mockReturnValue(
            Promise.resolve(<RequestResponse<Project>>{isSuccess: false})
        );

        const commit = jest.fn();

        const project: Project = {
            name: '',
            id: '',
            description: '',
            isSelected: false,
            createdAt: ''
        };

        const dispatchResponse = await projectsModule.actions.createProject({commit}, project);

        expect(dispatchResponse.isSuccess).toBeFalsy();
        expect(commit).toHaveBeenCalledTimes(0);
    });

    it('success select project', () => {
        const commit = jest.fn();

        projectsModule.actions.selectProject({commit}, 'id');

        expect(commit).toHaveBeenCalledWith(PROJECTS_MUTATIONS.SELECT, 'id');
    });

    it('success update project description', async () => {
        jest.spyOn(api, 'updateProjectRequest').mockReturnValue(Promise.resolve(<RequestResponse<null>>{isSuccess: true}));
        const commit = jest.fn();
        const project: Project = {
            name: '',
            id: 'id',
            description: 'desc',
            isSelected: false,
            createdAt: ''
        };

        const dispatchResponse = await projectsModule.actions.updateProject({commit}, project);

        expect(dispatchResponse.isSuccess).toBeTruthy();
        expect(commit).toBeCalledWith(PROJECTS_MUTATIONS.UPDATE, project);
    });

    it('error update project description', async () => {
        jest.spyOn(api, 'updateProjectRequest').mockReturnValue(Promise.resolve(<RequestResponse<null>>{isSuccess: false}));
        const commit = jest.fn();
        const project: Project = {
            name: '',
            id: '',
            description: '',
            isSelected: false,
            createdAt: ''
        };

        const dispatchResponse = await projectsModule.actions.updateProject({commit}, project);

        expect(dispatchResponse.isSuccess).toBeFalsy();
        expect(commit).toHaveBeenCalledTimes(0);
    });

    it('success delete project', async () => {
        jest.spyOn(api, 'deleteProjectRequest').mockReturnValue(Promise.resolve(<RequestResponse<null>>{isSuccess: true}));
        const commit = jest.fn();
        const project = 'id';

        const dispatchResponse = await projectsModule.actions.deleteProject({commit}, project);

        expect(dispatchResponse.isSuccess).toBeTruthy();
        expect(commit).toHaveBeenCalledWith(PROJECTS_MUTATIONS.DELETE, project);
    });

    it('error delete project', async () => {
        jest.spyOn(api, 'deleteProjectRequest').mockReturnValue(Promise.resolve(<RequestResponse<null>>{isSuccess: false}));
        const commit = jest.fn();

        const dispatchResponse = await projectsModule.actions.deleteProject({commit}, 'id');

        expect(dispatchResponse.isSuccess).toBeFalsy();
        expect(commit).toHaveBeenCalledTimes(0);
    });
});

describe('getters', () => {

    it('getter projects', () => {
        const state = {
            projects: [{
                name: '1',
                id: '1',
                companyName: '1',
                description: '1',
                isTermsAccepted: true,
                createdAt: '1',
            }],
            selectedProject: {
                id: '1'
            }
        };
        const projectsGetterArray = projectsModule.getters.projects(state);

        expect(projectsGetterArray.length).toBe(1);

        const firstProject = projectsGetterArray[0];

        expect(firstProject.name).toBe('1');
        expect(firstProject.id).toBe('1');
        expect(firstProject.description).toBe('1');
        expect(firstProject.isTermsAccepted).toBe(true);
        expect(firstProject.createdAt).toBe('1');
    });

    it('getter projects', () => {
        const state = {
            projects: [{
                name: '1',
                id: '1',
                companyName: '1',
                description: '1',
                isTermsAccepted: true,
                createdAt: '1',
            }],
            selectedProject: {
                id: '2'
            }
        };
        const projectsGetterArray = projectsModule.getters.projects(state);

        expect(projectsGetterArray.length).toBe(1);

        const firstProject = projectsGetterArray[0];

        expect(firstProject.name).toBe('1');
        expect(firstProject.id).toBe('1');
        expect(firstProject.description).toBe('1');
        expect(firstProject.isTermsAccepted).toBe(true);
        expect(firstProject.createdAt).toBe('1');
    });

    it('getters selected project', () => {
        const state = {
            selectedProject: {
                name: '1',
                id: '1',
                description: '1',
                isTermsAccepted: true,
                createdAt: '1',
            }
        };
        const selectedProjectGetterObject = projectsModule.getters.selectedProject(state);

        expect(selectedProjectGetterObject.name).toBe('1');
        expect(selectedProjectGetterObject.id).toBe('1');
        expect(selectedProjectGetterObject.description).toBe('1');
        expect(selectedProjectGetterObject.isTermsAccepted).toBe(true);
        expect(selectedProjectGetterObject.createdAt).toBe('1');
    });
});
