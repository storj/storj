// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

class AppState {
    public isNavigationDrawerShown = true;
    public isUpgradeFlowDialogShown = false;
    public isAccountSetupDialogShown = false;
    public pathBeforeAccountPage: string | null = null;
    public isNavigating = false;
}

export const useAppStore = defineStore('vuetifyApp', () => {
    const state = reactive<AppState>(new AppState());

    function toggleNavigationDrawer(isShown?: boolean): void {
        state.isNavigationDrawerShown = isShown ?? !state.isNavigationDrawerShown;
    }

    function toggleUpgradeFlow(isShown?: boolean): void {
        state.isUpgradeFlowDialogShown = isShown ?? !state.isUpgradeFlowDialogShown;
    }

    function toggleAccountSetup(isShown?: boolean): void {
        state.isAccountSetupDialogShown = isShown ?? !state.isAccountSetupDialogShown;
    }

    function setPathBeforeAccountPage(path: string) {
        state.pathBeforeAccountPage = path;
    }

    function setIsNavigating(isLoading: boolean) {
        state.isNavigating = isLoading;
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
        toggleAccountSetup,
        setPathBeforeAccountPage,
        setIsNavigating,
        clear,
    };
});
