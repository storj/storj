// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import {
    Project,
    ProjectFields,
    ProjectLimits,
    ProjectsApi,
} from '@/types/projects';

export const PROJECTS_ACTIONS = {
    FETCH: 'fetchProjects',
    CREATE: 'createProject',
    SELECT: 'selectProject',
    UPDATE_NAME: 'updateProjectName',
    UPDATE_DESCRIPTION: 'updateProjectDescription',
    DELETE: 'deleteProject',
    CLEAR: 'clearProjects',
    GET_LIMITS: 'getProjectLimits',
};

export const PROJECTS_MUTATIONS = {
    ADD: 'CREATE_PROJECT',
    REMOVE: 'DELETE_PROJECT',
    UPDATE_PROJECT_NAME: 'UPDATE_PROJECT_NAME',
    UPDATE_PROJECT_DESCRIPTION: 'UPDATE_PROJECT_DESCRIPTION',
    SET_PROJECTS: 'SET_PROJECTS',
    SELECT_PROJECT: 'SELECT_PROJECT',
    CLEAR_PROJECTS: 'CLEAR_PROJECTS',
    SET_LIMITS: 'SET_PROJECT_LIMITS',
};

const defaultSelectedProject = new Project('', '', '', '', '', true);

export class ProjectsState {
    public projects: Project[] = [];
    public selectedProject: Project = defaultSelectedProject;
    public currentLimits: ProjectLimits = new ProjectLimits();
}

const {
    FETCH,
    CREATE,
    SELECT,
    UPDATE_NAME,
    UPDATE_DESCRIPTION,
    DELETE,
    CLEAR,
    GET_LIMITS,
} = PROJECTS_ACTIONS;

const {
    ADD,
    REMOVE,
    UPDATE_PROJECT_NAME,
    UPDATE_PROJECT_DESCRIPTION,
    SET_PROJECTS,
    SELECT_PROJECT,
    CLEAR_PROJECTS,
    SET_LIMITS,
} = PROJECTS_MUTATIONS;

export function makeProjectsModule(api: ProjectsApi): StoreModule<ProjectsState> {
    return {
        state: new ProjectsState(),
        mutations: {
            [ADD](state: ProjectsState, createdProject: Project): void {
                state.projects.push(createdProject);
            },
            [SET_PROJECTS](state: ProjectsState, projects: Project[]): void {
                state.projects = projects;

                if (!state.selectedProject.id) {
                    return;
                }

                const projectsCount = state.projects.length;

                for (let i = 0; i < projectsCount; i++) {
                    const project = state.projects[i];

                    if (project.id !== state.selectedProject.id) {
                        continue;
                    }

                    state.selectedProject = project;

                    return;
                }

                state.selectedProject = defaultSelectedProject;
            },
            [SELECT_PROJECT](state: ProjectsState, projectID: string): void {
                const selected = state.projects.find((project: Project) => project.id === projectID);

                if (!selected) {
                    return;
                }

                state.selectedProject = selected;
            },
            [UPDATE_PROJECT_NAME](state: ProjectsState, fieldsToUpdate: ProjectFields): void {
                state.selectedProject.name = fieldsToUpdate.name;
            },
            [UPDATE_PROJECT_DESCRIPTION](state: ProjectsState, fieldsToUpdate: ProjectFields): void {
                state.selectedProject.description = fieldsToUpdate.description;
            },
            [REMOVE](state: ProjectsState, projectID: string): void {
                state.projects = state.projects.filter(project => project.id !== projectID);

                if (state.selectedProject.id === projectID) {
                    state.selectedProject = new Project();
                }
            },
            [SET_LIMITS](state: ProjectsState, limits: ProjectLimits): void {
                state.currentLimits = limits;
            },
            [CLEAR_PROJECTS](state: ProjectsState): void {
                state.projects = [];
                state.selectedProject = defaultSelectedProject;
                state.currentLimits = new ProjectLimits();
            },
        },
        actions: {
            [FETCH]: async function ({commit}: any): Promise<Project[]> {
                const projects = await api.get();

                commit(SET_PROJECTS, projects);

                return projects;
            },
            [CREATE]: async function ({commit}: any, createProjectFields: ProjectFields): Promise<Project> {
                const project = await api.create(createProjectFields);

                commit(ADD, project);

                return project;
            },
            [SELECT]: function ({commit}: any, projectID: string): void {
                commit(SELECT_PROJECT, projectID);
            },
            [UPDATE_NAME]: async function ({commit, state}: any, fieldsToUpdate: ProjectFields): Promise<void> {
                await api.update(state.selectedProject.id, fieldsToUpdate.name, state.selectedProject.description);

                commit(UPDATE_PROJECT_NAME, fieldsToUpdate);
            },
            [UPDATE_DESCRIPTION]: async function ({commit, state}: any, fieldsToUpdate: ProjectFields): Promise<void> {
                await api.update(state.selectedProject.id, state.selectedProject.name, fieldsToUpdate.description);

                commit(UPDATE_PROJECT_DESCRIPTION, fieldsToUpdate);
            },
            [DELETE]: async function ({commit}: any, projectID: string): Promise<void> {
                await api.delete(projectID);

                commit(REMOVE, projectID);
            },
            [GET_LIMITS]: async function ({commit}: any, projectID: string): Promise<ProjectLimits> {
                const limits = await api.getLimits(projectID);

                commit(SET_LIMITS, limits);

                return limits;
            },
            [CLEAR]: function({commit}: any): void {
                commit(CLEAR_PROJECTS);
            },
        },
        getters: {
            projects: (state: ProjectsState): Project[] => {
                return state.projects.map((project: Project) => {
                    if (project.id === state.selectedProject.id) {
                        project.isSelected = true;
                    }

                    return project;
                });
            },
            projectsWithoutSelected: (state: ProjectsState): Project[] => {
                return state.projects.filter((project: Project) => {
                    return project.id !== state.selectedProject.id;
                });
            },
            selectedProject: (state: ProjectsState): Project => state.selectedProject,
            projectsCount: (state: ProjectsState, getters: any): number => {
                let projectsCount: number = 0;

                state.projects.map((project: Project) => {
                    if (project.ownerId === getters.user.id) {
                        projectsCount++;
                    }
                });

                return projectsCount;
            },
        },
    };
}
