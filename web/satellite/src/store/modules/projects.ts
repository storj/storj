// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { PROJECTS_MUTATIONS } from '../mutationConstants';
import { createProjectRequest, deleteProjectRequest, fetchProjectsRequest, updateProjectRequest } from '@/api/projects';

export const projectsModule = {
    state: {
        projects: [],
        selectedProject: {
            name: 'Choose Project',
            id: '',
            companyName: '',
            description: '',
            isTermsAccepted: false,
            createdAt: '',
        }
    },
    mutations: {
        [PROJECTS_MUTATIONS.CREATE](state: any, createdProject: Project): void {
            state.projects.push(createdProject);
        },
        [PROJECTS_MUTATIONS.FETCH](state: any, projects: Project[]): void {
            state.projects = projects;
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

            if (state.selectedProject.id === updateProjectModel.id) {
                state.selectedProject.description = updateProjectModel.description;
            }
        },
        [PROJECTS_MUTATIONS.DELETE](state: any, projectID: string): void {
            if (state.selectedProject.id === projectID) {
                state.selectedProject.id = '';
            }
        },
        [PROJECTS_MUTATIONS.CLEAR](state: any): void {
            state.projects = [];
            state.selectedProject = {
                name: 'Choose Project',
                id: '',
                companyName: '',
                description: '',
                isTermsAccepted: false,
                createdAt: '',
            };
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
        createProject: async function ({commit}: any, project: Project): Promise<RequestResponse<Project>> {
            let response = await createProjectRequest(project);

            if (response.isSuccess) {
                commit(PROJECTS_MUTATIONS.CREATE, response.data);
            }

            return response;
        },
        selectProject: function ({commit}: any, projectID: string) {
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
        clearProjects: function({commit}: any) {
            commit(PROJECTS_MUTATIONS.CLEAR);
        }
    },
    getters: {
        projects: (state: any) => {
            return state.projects.map((project: any) => {
                if (project.id === state.selectedProject.id) {
                    project.isSelected = true;
                }

                return project;
            });
        },
        selectedProject: (state: any) => state.selectedProject,
    },
};
