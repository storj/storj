// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/store';
import { CreateProjectModel, Project, ProjectsApi, UpdateProjectModel } from '@/types/projects';

export const PROJECTS_ACTIONS = {
    FETCH: 'fetchProjects',
    CREATE: 'createProject',
    SELECT: 'selectProject',
    UPDATE: 'updateProject',
    DELETE: 'deleteProject',
    CLEAR: 'clearProjects',
};

export const PROJECTS_MUTATIONS = {
    ADD: 'CREATE_PROJECT',
    REMOVE: 'DELETE_PROJECT',
    UPDATE_PROJECT: 'UPDATE_PROJECT',
    SET_PROJECTS: 'SET_PROJECTS',
    SELECT_PROJECT: 'SELECT_PROJECT',
    CLEAR_PROJECTS: 'CLEAR_PROJECTS',
};

const defaultSelectedProject = new Project('', '', '', '', '', true);

class ProjectsState {
    public projects: Project[] = [];
    public selectedProject: Project = defaultSelectedProject;
}

const {
    FETCH,
    CREATE,
    SELECT,
    UPDATE,
    DELETE,
    CLEAR,
} = PROJECTS_ACTIONS;

const {
    ADD,
    REMOVE,
    UPDATE_PROJECT,
    SET_PROJECTS,
    SELECT_PROJECT,
    CLEAR_PROJECTS,
} = PROJECTS_MUTATIONS;

export function makeProjectsModule(api: ProjectsApi): StoreModule<ProjectsState> {
    return {
        state: new ProjectsState(),
        mutations: {
            [ADD](state: any, createdProject: Project): void {
                state.projects.push(createdProject);
            },
            [SET_PROJECTS](state: any, projects: Project[]): void {
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
            [SELECT_PROJECT](state: any, projectID: string): void {
                const selected = state.projects.find((project: any) => project.id === projectID);

                if (!selected) {
                    return;
                }

                state.selectedProject = selected;
            },
            [UPDATE_PROJECT](state: any, updateProjectModel: UpdateProjectModel): void {
                const selected = state.projects.find((project: any) => project.id === updateProjectModel.id);
                if (!selected) {
                    return;
                }

                selected.description = updateProjectModel.description;
            },
            [REMOVE](state: any, projectID: string): void {
                state.projects = state.projects.filter(project => project.id !== projectID);

                if (state.selectedProject.id === projectID) {
                    state.selectedProject = new Project();
                }
            },
            [CLEAR_PROJECTS](state: ProjectsState): void {
                state.projects = [];
                state.selectedProject = defaultSelectedProject;
            },
        },
        actions: {
            [FETCH]: async function ({commit}: any): Promise<Project[]> {
                const projects = await api.get();

                commit(SET_PROJECTS, projects);

                return projects;
            },
            [CREATE]: async function ({commit}: any, createProjectModel: CreateProjectModel): Promise<Project> {
                const project = await api.create(createProjectModel);

                commit(ADD, project);

                return project;
            },
            [SELECT]: function ({commit}: any, projectID: string): void {
                commit(SELECT_PROJECT, projectID);
            },
            [UPDATE]: async function ({commit}: any, updateProjectModel: UpdateProjectModel): Promise<void> {
                await api.update(updateProjectModel.id, updateProjectModel.description);

                commit(UPDATE_PROJECT, updateProjectModel);
            },
            [DELETE]: async function ({commit}: any, projectID: string): Promise<void> {
                await api.delete(projectID);

                commit(REMOVE, projectID);
            },
            [CLEAR]: function({commit}: any): void {
                commit(CLEAR_PROJECTS);
            },
        },
        getters: {
            projects: (state: any): Project[] => {
                return state.projects.map((project: any) => {
                    if (project.id === state.selectedProject.id) {
                        project.isSelected = true;
                    }

                    return project;
                });
            },
            selectedProject: (state: any): Project => state.selectedProject,
        },
    };
}
