// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import {
    ChangeHistoryHttpApiV1,
    ChangeLog,
    DisableProjectRequest,
    Project,
    ProjectEntitlements,
    ProjectLimitsUpdateRequest,
    ProjectManagementHttpApiV1,
    ProjectStatusInfo,
    UpdateProjectEntitlementsRequest,
    UpdateProjectRequest,
} from '@/api/client.gen';
import { ItemType } from '@/types/common';

class ProjectsState {
    currentProject: Project | null = null;
    public projectStatuses: ProjectStatusInfo[] = [];
}

export const useProjectsStore = defineStore('projects', () => {
    const state = reactive<ProjectsState>(new ProjectsState());

    const changeHistoryApi = new ChangeHistoryHttpApiV1();
    const projectApi = new ProjectManagementHttpApiV1();

    async function updateCurrentProject(project: string | Project): Promise<void> {
        if (typeof project === 'string') {
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
        return await projectApi.updateProjectLimits(limits, id);
    }

    async function updateProject(projectID: string, request: UpdateProjectRequest): Promise<Project> {
        return await projectApi.updateProject(request, projectID);
    }

    async function disableProject(projectID: string, setPendingDeletion: boolean, reason: string): Promise<void> {
        const request = new DisableProjectRequest();
        request.reason = reason;
        request.setPendingDeletion = setPendingDeletion;
        return await projectApi.disableProject(request, projectID);
    }

    async function getProjectStatuses(): Promise<void> {
        if (state.projectStatuses.length) {
            return;
        }
        state.projectStatuses = await projectApi.getProjectStatuses();
    }

    async function updateEntitlements(projectID: string, request: UpdateProjectEntitlementsRequest): Promise<ProjectEntitlements> {
        return await projectApi.updateProjectEntitlements(request, projectID);
    }

    async function getHistory(projectID: string, exact = true): Promise<ChangeLog[]> {
        return await changeHistoryApi.getChangeHistory(`${exact}`, ItemType.Project, projectID);
    }

    return {
        state,
        getProject,
        updateCurrentProject,
        disableProject,
        clearCurrentProject,
        updateProjectLimits,
        updateProject,
        getProjectStatuses,
        updateEntitlements,
        getHistory,
    };
});
