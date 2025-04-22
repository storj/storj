// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import { LocalData } from '@/utils/localData';
import { useAnalyticsStore } from '@/store/modules/analyticsStore';
import { useUsersStore } from '@/store/modules/usersStore';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';

class AppState {
    public hasJustLoggedIn = false;
    public isUploadingModal = false;
    public error: ErrorPageState = new ErrorPageState();
    public isProjectTableViewEnabled = LocalData.getProjectTableViewEnabled();
    public isBrowserCardViewEnabled = LocalData.getBrowserCardViewEnabled();
    public isNavigationDrawerShown = true;
    public isUpgradeFlowDialogShown = false;
    public isExpirationDialogShown = false;
    public isProjectPassphraseDialogShown = false;
    public managedPassphraseNotRetrievable = false;
    public managedPassphraseErrorDialogShown = false;
    public pathBeforeAccountPage: string | null = null;
    public isNavigating = false;
}

class ErrorPageState {
    constructor(
        public statusCode = 0,
        public fatal = false,
        public visible = false,
    ) { }
}

export const useAppStore = defineStore('app', () => {
    const state = reactive<AppState>(new AppState());

    const analyticsStore = useAnalyticsStore();
    const userStore = useUsersStore();

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

    function toggleBrowserCardViewEnabled(isBrowserCardViewEnabled: boolean | null = null): void {
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

    function toggleNavigationDrawer(isShown?: boolean): void {
        state.isNavigationDrawerShown = isShown ?? !state.isNavigationDrawerShown;
    }

    function toggleUpgradeFlow(isShown?: boolean): void {
        state.isUpgradeFlowDialogShown = isShown ?? !state.isUpgradeFlowDialogShown;
        if (state.isUpgradeFlowDialogShown) {
            analyticsStore.eventTriggered(AnalyticsEvent.UPGRADE_CLICKED, { expired: String(userStore.state.user.freezeStatus.trialExpiredFrozen) });
        }
    }

    function toggleExpirationDialog(isShown?: boolean): void {
        state.isExpirationDialogShown = isShown ?? !state.isExpirationDialogShown;
    }

    function toggleProjectPassphraseDialog(isShown?: boolean): void {
        state.isProjectPassphraseDialogShown = isShown ?? !state.isProjectPassphraseDialogShown;
    }

    function toggleManagedPassphraseErrorDialog(isShown?: boolean): void {
        state.managedPassphraseErrorDialogShown = isShown ?? !state.managedPassphraseErrorDialogShown;
    }

    function setManagedPassphraseNotRetrievable(notRetrievable: boolean) {
        state.managedPassphraseNotRetrievable = notRetrievable;
    }

    function setPathBeforeAccountPage(path: string) {
        state.pathBeforeAccountPage = path;
    }

    function setIsNavigating(isLoading: boolean) {
        state.isNavigating = isLoading;
    }

    function setErrorPage(statusCode: number, fatal = false): void {
        state.error = new ErrorPageState(statusCode, fatal, true);
    }

    function removeErrorPage(): void {
        state.error.visible = false;
    }

    function clear(): void {
        state.hasJustLoggedIn = false;
        state.isUploadingModal = false;
        state.error.visible = false;
        state.isProjectTableViewEnabled = false;
        LocalData.removeProjectTableViewConfig();
        state.isNavigationDrawerShown = true;
        state.isUpgradeFlowDialogShown = false;
        state.pathBeforeAccountPage = null;
        state.managedPassphraseNotRetrievable = false;
        state.managedPassphraseErrorDialogShown = false;
    }

    return {
        state,
        toggleProjectTableViewEnabled,
        toggleBrowserCardViewEnabled,
        hasProjectTableViewConfigured,
        toggleHasJustLoggedIn,
        toggleProjectPassphraseDialog,
        setManagedPassphraseNotRetrievable,
        toggleManagedPassphraseErrorDialog,
        toggleExpirationDialog,
        setUploadingModal,
        setErrorPage,
        removeErrorPage,
        toggleNavigationDrawer,
        toggleUpgradeFlow,
        setPathBeforeAccountPage,
        setIsNavigating,
        clear,
    };
});
