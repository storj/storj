// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import { OnboardingOS, PricingPlanInfo } from '@/types/common';
import { FetchState } from '@/utils/constants/fetchStateEnum';
import { ManageProjectPassphraseStep } from '@/types/managePassphrase';

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
    // activeModal could be of VueConstructor type or Object (for composition api components).
    public activeModal: unknown | null = null;
    // this field is mainly used on the all projects dashboard as an exit condition
    // for when the dashboard opens the pricing plan and the pricing plan navigates back repeatedly.
    public hasShownPricingPlan = false;
    public error: ErrorPageState = new ErrorPageState();
    public isLargeUploadNotificationShown = true;
    public isLargeUploadWarningNotificationShown = false;
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

    function updateActiveModal(modal: unknown): void {
        // modal could be of VueConstructor type or Object (for composition api components).
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

    function setOnboardingCleanAPIKey(apiKey: string): void {
        state.onbCleanApiKey = apiKey;
    }

    function setOnboardingOS(os: OnboardingOS): void {
        state.onbSelectedOs = os;
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
        state.error.visible = false;
    }

    return {
        state,
        toggleActiveDropdown,
        toggleSuccessfulPasswordReset,
        updateActiveModal,
        removeActiveModal,
        toggleHasJustLoggedIn,
        changeState,
        setOnboardingBackRoute,
        setOnboardingAPIKeyStepBackRoute,
        setOnboardingAPIKey,
        setOnboardingCleanAPIKey,
        setOnboardingOS,
        setPricingPlan,
        setManagePassphraseStep,
        setHasShownPricingPlan,
        setLargeUploadWarningNotification,
        setLargeUploadNotification,
        closeDropdowns,
        setErrorPage,
        removeErrorPage,
        clear,
    };
});
