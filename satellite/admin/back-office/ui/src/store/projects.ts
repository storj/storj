// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import {
    Project,
    ProjectLimitsUpdateRequest,
    ProjectManagementHttpApiV1,
} from '@/api/client.gen';

class ProjectsState {
    currentProject: Project | null = null;
}

export const useProjectsStore = defineStore('projects', () => {
    const state = reactive<ProjectsState>(new ProjectsState());

    const projectApi = new ProjectManagementHttpApiV1();

    async function updateCurrentProject(project: string | Project): Promise<void> {
        if (typeof project === 'string') {
            clearCurrentProject();
            state.currentProject = await getProject(project);
            return;
        }
        state.currentProject = project;
    }

    function clearCurrentProject(): void {
        state.currentProject = null;
    }

    async function getProject(id: string): Promise<Project> {
        return await projectApi.getProject(id);
    }

    async function updateProjectLimits(id: string, limits: ProjectLimitsUpdateRequest): Promise<Project> {
        limits.reason = 'TBD';
        return await projectApi.updateProjectLimits(limits, id);
    }

    return {
        state,
        getProject,
        updateCurrentProject,
        clearCurrentProject,
        updateProjectLimits,
    };
});
