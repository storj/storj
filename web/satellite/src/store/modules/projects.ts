// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { PROJECTS_MUTATIONS } from '../mutationConstants';
import { createProjectRequest, deleteProjectRequest, fetchProjectsRequest, updateProjectRequest } from '@/api/projects';
import { RequestResponse } from '@/types/response';
import { CreateProjectModel, Project, UpdateProjectModel } from '@/types/projects';

export const projectsModule = {
    state: {
        projects: [],
        selectedProject: new Project(true),
    },
    mutations: {
        [PROJECTS_MUTATIONS.CREATE](state: any, createdProject: Project): void {
            state.projects.push(createdProject);
        },
        [PROJECTS_MUTATIONS.FETCH](state: any, projects: Project[]): void {
            state.projects = projects;

            if (!state.selectedProject.id) {
                return;
            }

            let projectsCount = state.projects.length;

            for (let i = 0; i < projectsCount; i++) {
                let project = state.projects[i];

                if (project.id !== state.selectedProject.id) {
                    continue;
                }

                state.selectedProject = project;

                return;
            }

            state.selectedProject = new Project(true);
        },
        [PROJECTS_MUTATIONS.SELECT](state: any, projectID: string): void {
            const selected = state.projects.find((project: any) => project.id === projectID);

            if (!selected) {
                return;
            }

            state.selectedProject = selected;
        },
        [PROJECTS_MUTATIONS.UPDATE](state: any, updateProjectModel: UpdateProjectModel): void {
            const selected = state.projects.find((project: any) => project.id === updateProjectModel.id);
            if (!selected) {
                return;
            }

            selected.description = updateProjectModel.description;
        },
        [PROJECTS_MUTATIONS.DELETE](state: any, projectID: string): void {
            state.projects = state.projects.filter(proj => proj.id !== projectID);

            if (state.selectedProject.id === projectID) {
                state.selectedProject = new Project(true);
            }
        },
        [PROJECTS_MUTATIONS.CLEAR](state: any): void {
            state.projects = [];
            state.selectedProject = new Project(true);
        },
    },
    actions: {
        fetchProjects: async function ({commit}: any): Promise<RequestResponse<Project[]>> {
            let response: RequestResponse<Project[]> = await fetchProjectsRequest();

            if (response.isSuccess) {
                commit(PROJECTS_MUTATIONS.FETCH, response.data);
            }

            return response;
        },
        createProject: async function ({commit}: any, createProjectModel: CreateProjectModel): Promise<RequestResponse<Project>> {
            let response = await createProjectRequest(createProjectModel);

            if (response.isSuccess) {
                commit(PROJECTS_MUTATIONS.CREATE, response.data);
            }

            return response;
        },
        selectProject: function ({commit}: any, projectID: string): void {
            commit(PROJECTS_MUTATIONS.SELECT, projectID);
        },
        updateProject: async function ({commit}: any, updateProjectModel: UpdateProjectModel): Promise<RequestResponse<null>> {
            let response = await updateProjectRequest(updateProjectModel.id, updateProjectModel.description);

            if (response.isSuccess) {
                commit(PROJECTS_MUTATIONS.UPDATE, updateProjectModel);
            }

            return response;
        },
        deleteProject: async function ({commit}: any, projectID: string): Promise<RequestResponse<null>> {
            let response = await deleteProjectRequest(projectID);

            if (response.isSuccess) {
                commit(PROJECTS_MUTATIONS.DELETE, projectID);
            }

            return response;
        },
        clearProjects: function({commit}: any): void {
            commit(PROJECTS_MUTATIONS.CLEAR);
        }
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
