// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    PROJECTS_MUTATIONS
} from "../mutationConstants";
import { createProject, fetchProjects } from "@/api/projects";

export const projectsModule = {
    state: {
        projects: [],
        selectedProject: {
            name: "Choose Project",
            id: "",
            description: "",
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
    },
    actions: {
        fetchProjects: async function({commit}: any) {
            let response = await fetchProjects();

            if (!response || !response.data) {
                //TODO: popup error here
                console.log("error during project fetching!");
                return;
            }

            commit(PROJECTS_MUTATIONS.FETCH, response.data.myProjects);

        },
        createProject: async function({commit}: any, project: Project) {
            let response = await createProject(project);
            if(!response) {
                //TODO: popup error here
                console.log("error during project creation!");
                return;
            }

            commit(PROJECTS_MUTATIONS.CREATE, response);
        },
        selectProject: function({commit}: any, projectID: string) {
            commit(PROJECTS_MUTATIONS.SELECT, projectID);
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
    }
};
