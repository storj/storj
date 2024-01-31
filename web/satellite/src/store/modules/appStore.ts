// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import { ManageProjectPassphraseStep } from '@/types/managePassphrase';
import { LocalData } from '@/utils/localData';

class AppState {
    public hasJustLoggedIn = false;
    public managePassphraseStep: ManageProjectPassphraseStep | undefined = undefined;
    public isUploadingModal = false;
    public error: ErrorPageState = new ErrorPageState();
    public isProjectTableViewEnabled = LocalData.getProjectTableViewEnabled();
    public isBrowserCardViewEnabled = LocalData.getBrowserCardViewEnabled();
}

class ErrorPageState {
    constructor(
        public statusCode = 0,
        public fatal = false,
        public visible = false,
    ) {}
}

export const useAppStore = defineStore('app', () => {
    const state = reactive<AppState>(new AppState());

    function toggleHasJustLoggedIn(hasJustLoggedIn: boolean | null = null): void {
        if (hasJustLoggedIn === null) {
            state.hasJustLoggedIn = !state.hasJustLoggedIn;
            return;
        }
        state.hasJustLoggedIn = hasJustLoggedIn;
    }

    function hasProjectTableViewConfigured(): boolean {
        return LocalData.hasProjectTableViewConfigured();
    }

    function toggleProjectTableViewEnabled(isProjectTableViewEnabled: boolean | null = null): void {
        if (isProjectTableViewEnabled === null) {
            state.isProjectTableViewEnabled = !state.isProjectTableViewEnabled;
        } else {
            state.isProjectTableViewEnabled = isProjectTableViewEnabled;
        }
        LocalData.setProjectTableViewEnabled(state.isProjectTableViewEnabled);
    }

    function toggleBrowserTableViewEnabled(isBrowserCardViewEnabled: boolean | null = null): void {
        if (isBrowserCardViewEnabled === null) {
            state.isBrowserCardViewEnabled = !state.isBrowserCardViewEnabled;
        } else {
            state.isBrowserCardViewEnabled = isBrowserCardViewEnabled;
        }
        LocalData.setBrowserCardViewEnabled(state.isBrowserCardViewEnabled);
    }

    function setUploadingModal(value: boolean): void {
        state.isUploadingModal = value;
    }

    function setManagePassphraseStep(step: ManageProjectPassphraseStep | undefined): void {
        state.managePassphraseStep = step;
    }

    function setErrorPage(statusCode: number, fatal = false): void {
        state.error = new ErrorPageState(statusCode, fatal, true);
    }

    function removeErrorPage(): void {
        state.error.visible = false;
    }

    function clear(): void {
        state.hasJustLoggedIn = false;
        state.managePassphraseStep = undefined;
        state.isUploadingModal = false;
        state.error.visible = false;
        state.isProjectTableViewEnabled = false;
        LocalData.removeProjectTableViewConfig();
    }

    return {
        state,
        toggleProjectTableViewEnabled,
        toggleBrowserCardViewEnabled: toggleBrowserTableViewEnabled,
        hasProjectTableViewConfigured,
        toggleHasJustLoggedIn,
        setManagePassphraseStep,
        setUploadingModal,
        setErrorPage,
        removeErrorPage,
        clear,
    };
});
