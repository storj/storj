// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import { PlacementInfo, PlacementManagementHttpApiV1, Project, ProjectManagementHttpApiV1, UserAccount, UserManagementHttpApiV1 } from '@/api/client.gen';

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

    return {
        state,
        getUserByEmail,
        clearUser,
        getPlacements,
        getPlacementText,
        selectProject,
    };
});
