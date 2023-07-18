// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

class AppState {
    public isNavigationDrawerShown = true;
}

export const useAppStore = defineStore('app', () => {
    const state = reactive<AppState>(new AppState());

    function toggleNavigationDrawer(): void {
        state.isNavigationDrawerShown = !state.isNavigationDrawerShown;
    }

    return {
        state,
        toggleNavigationDrawer,
    };
});
