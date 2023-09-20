// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

class AppState {
    public isNavigationDrawerShown = true;
    public isUpgradeFlowDialogShown = false;
    public pathBeforeAccountPage: string | null = null;
}

export const useAppStore = defineStore('vuetifyApp', () => {
    const state = reactive<AppState>(new AppState());

    function toggleNavigationDrawer(isShown?: boolean): void {
        state.isNavigationDrawerShown = isShown ?? !state.isNavigationDrawerShown;
    }

    function toggleUpgradeFlow(isShown?: boolean): void {
        state.isUpgradeFlowDialogShown = isShown ?? !state.isUpgradeFlowDialogShown;
    }

    function setPathBeforeAccountPage(path: string) {
        state.pathBeforeAccountPage = path;
    }

    function clear(): void {
        state.isNavigationDrawerShown = true;
        state.isUpgradeFlowDialogShown = false;
        state.pathBeforeAccountPage = null;
    }

    return {
        state,
        toggleNavigationDrawer,
        toggleUpgradeFlow,
        setPathBeforeAccountPage,
        clear,
    };
});
