// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    PROJECTS_MUTATIONS
} from "../mutationConstants";
import { createProject, fetchProjects, updateProject, deleteProject } from "@/api/projects";

export const projectsModule = {
    state: {
        projects: [],
        selectedProject: {
            name: "Choose Project",
            id: "",
            ownerName: "",
            companyName: "",
            description: "",
            isTermsAccepted: false,
            createdAt: "",
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
                return
            }

            state.selectedProject = selected;
        },
        [PROJECTS_MUTATIONS.UPDATE](state: any, updateProjectModel: UpdateProjectModel): void {
            const selected = state.projects.find((project: any) => project.id === updateProjectModel.id);
            if (!selected) {
                // TODO: notify about error
                return
            }

            selected.description = updateProjectModel.description;

            if (state.selectedProject.id === updateProjectModel.id) {
                state.selectedProject.description = updateProjectModel.description;
            }
        },
        [PROJECTS_MUTATIONS.DELETE](state: any, projectID: string): void {
            if (state.selectedProject.id === projectID) {
                state.selectedProject.id = "";
            }
        },
    },
    actions: {
        fetchProjects: async function({commit}: any): Promise<boolean> {
            let response = await fetchProjects();

            if (!response || !response.data) {
                return false;
            }

            commit(PROJECTS_MUTATIONS.FETCH, response.data.myProjects);

            return true;
        },
        createProject: async function({commit}: any, project: Project): Promise<boolean> {
            let response = await createProject(project);

            console.log("in action", project);

            if(!response || response.errors) {
                return false;
            }

            commit(PROJECTS_MUTATIONS.CREATE, response);

            return true;
        },
        selectProject: function({commit}: any, projectID: string) {
            commit(PROJECTS_MUTATIONS.SELECT, projectID);
        },
        updateProjectDescription: async function({commit}: any, updateProjectModel: UpdateProjectModel): Promise<boolean> {
            let response = await updateProject(updateProjectModel.id, updateProjectModel.description);

            if (!response || response.errors) {
                return false;
            }

            commit(PROJECTS_MUTATIONS.UPDATE, updateProjectModel);

            return true;
        },
        deleteProject: async function({commit}: any, projectID: string) : Promise<boolean> {
            let response = await deleteProject(projectID);

            if (!response || response.errors) {
                return false;
            }

            commit(PROJECTS_MUTATIONS.FETCH);
            commit(PROJECTS_MUTATIONS.DELETE, projectID);

            return true;
        },
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
