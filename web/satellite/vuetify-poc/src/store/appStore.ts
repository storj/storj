// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

class AppState {
    public isNavigationDrawerShown = true;
    public pathBeforeAccountPage: string | null = null;
}

export const useAppStore = defineStore('vuetifyApp', () => {
    const state = reactive<AppState>(new AppState());

    function toggleNavigationDrawer(isShown?: boolean): void {
        state.isNavigationDrawerShown = isShown ?? !state.isNavigationDrawerShown;
    }

    function setPathBeforeAccountPage(path: string) {
        state.pathBeforeAccountPage = path;
    }

    function clear(): void {
        state.isNavigationDrawerShown = true;
        state.pathBeforeAccountPage = null;
    }

    return {
        state,
        toggleNavigationDrawer,
        setPathBeforeAccountPage,
        clear,
    };
});
