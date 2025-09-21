// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import {
    PlacementInfo,
    PlacementManagementHttpApiV1,
    Settings,
    SettingsHttpApiV1,
} from '@/api/client.gen';

class AppState {
    public placements: PlacementInfo[];
    public settings: Settings;
}

export const useAppStore = defineStore('app', () => {
    const state = reactive<AppState>(new AppState());

    const placementApi = new PlacementManagementHttpApiV1();
    const settingsApi = new SettingsHttpApiV1();

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

    async function getSettings(): Promise<void> {
        state.settings = await settingsApi.get();
    }

    return {
        state,
        getPlacements,
        getPlacementText,
        getSettings,
    };
});
