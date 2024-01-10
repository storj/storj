// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import { PlacementInfo, PlacementManagementHttpApiV1, Project, ProjectLimitsUpdate, ProjectManagementHttpApiV1, UserAccount, UserManagementHttpApiV1 } from '@/api/client.gen';

class AppState {
    public placements: PlacementInfo[];
    public userAccount: UserAccount | null = null;
    public selectedProject: Project | null = null;
}

export const useAppStore = defineStore('app', () => {
    const state = reactive<AppState>(new AppState());

    const userApi = new UserManagementHttpApiV1();
    const placementApi = new PlacementManagementHttpApiV1();
    const projectApi = new ProjectManagementHttpApiV1();

    async function getUserByEmail(email: string): Promise<void> {
        state.userAccount = await userApi.getUserByEmail(email);
    }

    function clearUser(): void {
        state.userAccount = null;
    }

    async function getPlacements(): Promise<void> {
        state.placements = await placementApi.getPlacements();
    }

    function getPlacementText(code: number): string {
        for (const placement of state.placements) {
            if (placement.id === code) {
                if (placement.location) {
                    return placement.location;
                }
                break;
            }
        }
        return `Unknown (${code})`;
    }

    async function selectProject(id: string): Promise<void> {
        state.selectedProject = await projectApi.getProject(id);
    }

    async function updateProjectLimits(id: string, limits: ProjectLimitsUpdate): Promise<void> {
        await projectApi.updateProjectLimits(limits, id);
        if (state.selectedProject && state.selectedProject.id === id) {
            state.selectedProject.maxBuckets = limits.maxBuckets;
            state.selectedProject.storageLimit = limits.storageLimit;
            state.selectedProject.bandwidthLimit = limits.bandwidthLimit;
            state.selectedProject.segmentLimit = limits.segmentLimit;
            state.selectedProject.rateLimit = limits.rateLimit;
            state.selectedProject.burstLimit = limits.burstLimit;
        }
        if (state.userAccount && state.userAccount.projects) {
            const updatedData = {
                storageLimit: limits.storageLimit,
                bandwidthLimit: limits.bandwidthLimit,
                segmentLimit: limits.segmentLimit,
            };
            state.userAccount.projects.map((item) => (item.id === id ? { ...item, updatedData } : item));
        }
    }

    return {
        state,
        getUserByEmail,
        clearUser,
        getPlacements,
        getPlacementText,
        selectProject,
        updateProjectLimits,
    };
});
