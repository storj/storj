// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    PROJECTS_MUTATIONS
} from "../mutationConstants";
import { createProject } from "@/utils/qraphql/createProjectsQuery";

export const projectsModule = {
    state: {
        projects: [],
        selectedProject: {
            name: "",
            id: "",
        }
    },

    mutations: {
        [PROJECTS_MUTATIONS.CREATE](state: any, createdProject: Project): void {
            state.projects.push(createdProject)
        },
        [PROJECTS_MUTATIONS.FETCH](state: any, projects: Project[]): void {
            state.projects = projects
        },
    },

    actions: {
        fetchProjects: async function({commit}: any) {
            commit(PROJECTS_MUTATIONS.FETCH, )
        },
        createProject: async function({commit}: any, project: Project) {
            let response = createProject(project);

            if(response) {
                commit(PROJECTS_MUTATIONS.CREATE, response)
            }
        }

    },

    getters: {

    },
};
