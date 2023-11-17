// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import { PlacementInfo, PlacementManagementHttpApiV1, User, UserManagementHttpApiV1 } from '@/api/client.gen';

class AppState {
    public placements: PlacementInfo[];
    public user: User | null = null;
}

export const useAppStore = defineStore('app', () => {
    const state = reactive<AppState>(new AppState());

    const userApi = new UserManagementHttpApiV1();
    const placementApi = new PlacementManagementHttpApiV1();

    async function getUserByEmail(email: string): Promise<void> {
        state.user = await userApi.getUserByEmail(email);
    }

    function clearUser(): void {
        state.user = null;
    }

    async function getPlacements(): Promise<void> {
        state.placements = await placementApi.getPlacements();
    }

    return {
        state,
        getUserByEmail,
        clearUser,
        getPlacements,
    };
});
