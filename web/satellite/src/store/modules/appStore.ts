// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { Component, reactive } from 'vue';
import { defineStore } from 'pinia';

import { OnboardingOS, PricingPlanInfo } from '@/types/common';
import { FetchState } from '@/utils/constants/fetchStateEnum';
import { ManageProjectPassphraseStep } from '@/types/managePassphrase';
import { LocalData } from '@/utils/localData';
import { LimitToChange } from '@/types/projects';

class AppState {
    public fetchState = FetchState.LOADING;
    public isSuccessfulPasswordResetShown = false;
    public hasJustLoggedIn = false;
    public onbAGStepBackRoute = '';
    public onbAPIKeyStepBackRoute = '';
    public onbCleanApiKey = '';
    public onbApiKey = '';
    public setDefaultPaymentMethodID = '';
    public deletePaymentMethodID = '';
    public onbSelectedOs: OnboardingOS | null = null;
    public selectedPricingPlan: PricingPlanInfo | null = null;
    public managePassphraseStep: ManageProjectPassphraseStep | undefined = undefined;
    public activeDropdown = 'none';
    public activeModal: Component | null = null;
    public isUploadingModal = false;
    public isGalleryView = false;
    // this field is mainly used on the all projects dashboard as an exit condition
    // for when the dashboard opens the pricing plan and the pricing plan navigates back repeatedly.
    public hasShownPricingPlan = false;
    public error: ErrorPageState = new ErrorPageState();
    public isLargeUploadNotificationShown = true;
    public isLargeUploadWarningNotificationShown = false;
    public activeChangeLimit: LimitToChange = LimitToChange.Storage;
    public isProjectTableViewEnabled = LocalData.getProjectTableViewEnabled();
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

    function toggleActiveDropdown(dropdown: string): void {
        if (state.activeDropdown === dropdown) {
            state.activeDropdown = 'none';
            return;
        }

        state.activeDropdown = dropdown;
    }

    function toggleSuccessfulPasswordReset(): void {
        if (!state.isSuccessfulPasswordResetShown) {
            state.activeDropdown = 'none';
        }

        state.isSuccessfulPasswordResetShown = !state.isSuccessfulPasswordResetShown;
    }

    function updateActiveModal(modal: Component): void {
        if (state.activeModal === modal) {
            state.activeModal = null;
            return;
        }
        state.activeModal = modal;
    }

    function removeActiveModal(): void {
        state.activeModal = null;
    }

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

    function changeState(newFetchState: FetchState): void {
        state.fetchState = newFetchState;
    }

    function setOnboardingBackRoute(backRoute: string): void {
        state.onbAGStepBackRoute = backRoute;
    }

    function setOnboardingAPIKeyStepBackRoute(backRoute: string): void {
        state.onbAPIKeyStepBackRoute = backRoute;
    }

    function setOnboardingAPIKey(apiKey: string): void {
        state.onbApiKey = apiKey;
    }

    function setUploadingModal(value: boolean): void {
        state.isUploadingModal = value;
    }

    function setOnboardingCleanAPIKey(apiKey: string): void {
        state.onbCleanApiKey = apiKey;
    }

    function setOnboardingOS(os: OnboardingOS): void {
        state.onbSelectedOs = os;
    }

    function setActiveChangeLimit(limit: LimitToChange): void {
        state.activeChangeLimit = limit;
    }

    function setPricingPlan(plan: PricingPlanInfo): void {
        state.selectedPricingPlan = plan;
    }

    function setManagePassphraseStep(step: ManageProjectPassphraseStep | undefined): void {
        state.managePassphraseStep = step;
    }

    function setHasShownPricingPlan(value: boolean): void {
        state.hasShownPricingPlan = value;
    }

    function setLargeUploadWarningNotification(value: boolean): void {
        state.isLargeUploadWarningNotificationShown = value;
    }

    function setLargeUploadNotification(value: boolean): void {
        state.isLargeUploadNotificationShown = value;
    }

    function setGalleryView(value: boolean): void {
        state.isGalleryView = value;
    }

    function closeDropdowns(): void {
        state.activeDropdown = '';
    }

    function setErrorPage(statusCode: number, fatal = false): void {
        state.error = new ErrorPageState(statusCode, fatal, true);
    }

    function removeErrorPage(): void {
        state.error.visible = false;
    }

    function clear(): void {
        state.activeModal = null;
        state.isSuccessfulPasswordResetShown = false;
        state.hasJustLoggedIn = false;
        state.onbAGStepBackRoute = '';
        state.onbAPIKeyStepBackRoute = '';
        state.onbCleanApiKey = '';
        state.onbApiKey = '';
        state.setDefaultPaymentMethodID = '';
        state.deletePaymentMethodID = '';
        state.onbSelectedOs = null;
        state.managePassphraseStep = undefined;
        state.selectedPricingPlan = null;
        state.hasShownPricingPlan = false;
        state.activeDropdown = '';
        state.isUploadingModal = false;
        state.error.visible = false;
        state.isGalleryView = false;
        state.isProjectTableViewEnabled = false;
        LocalData.removeProjectTableViewConfig();
    }

    return {
        state,
        toggleActiveDropdown,
        toggleSuccessfulPasswordReset,
        toggleProjectTableViewEnabled,
        hasProjectTableViewConfigured,
        updateActiveModal,
        removeActiveModal,
        toggleHasJustLoggedIn,
        changeState,
        setOnboardingBackRoute,
        setOnboardingAPIKeyStepBackRoute,
        setOnboardingAPIKey,
        setOnboardingCleanAPIKey,
        setOnboardingOS,
        setActiveChangeLimit,
        setPricingPlan,
        setGalleryView,
        setManagePassphraseStep,
        setHasShownPricingPlan,
        setUploadingModal,
        setLargeUploadWarningNotification,
        setLargeUploadNotification,
        closeDropdowns,
        setErrorPage,
        removeErrorPage,
        clear,
    };
});
