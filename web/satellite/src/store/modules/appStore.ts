// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { reactive } from 'vue';
import { defineStore } from 'pinia';

import { OnboardingOS, PartneredSatellite, PricingPlanInfo } from '@/types/common';
import { FetchState } from '@/utils/constants/fetchStateEnum';
import { ManageProjectPassphraseStep } from '@/types/managePassphrase';
import { MetaUtils } from '@/utils/meta';
import { FrontendConfig } from '@/types/config.gen';
import { FrontendConfigApi } from '@/types/config';
import { FrontendConfigHttpApi } from '@/api/config';

class ViewsState {
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
}

export class State {
    public viewsState: ViewsState = new ViewsState();
    public satelliteName = '';
    public partneredSatellites = new Array<PartneredSatellite>();
    public isBetaSatellite = false;
    public couponCodeBillingUIEnabled = false;
    public couponCodeSignupUIEnabled = false;
    public isAllProjectsDashboard = MetaUtils.getMetaContent('all-projects-dashboard') === 'true';
    public config: FrontendConfig = new FrontendConfig();
}

export const useAppStore = defineStore('app', () => {
    const state = reactive<State>(new State());

    const configApi: FrontendConfigApi = new FrontendConfigHttpApi();

    async function getConfig(): Promise<FrontendConfig> {
        const result = await configApi.get();

        state.config = result;

        return result;
    }

    function toggleActiveDropdown(dropdown: string): void {
        if (state.viewsState.activeDropdown === dropdown) {
            state.viewsState.activeDropdown = 'none';
            return;
        }

        state.viewsState.activeDropdown = dropdown;
    }

    function toggleSuccessfulPasswordReset(): void {
        if (!state.viewsState.isSuccessfulPasswordResetShown) {
            state.viewsState.activeDropdown = 'none';
        }

        state.viewsState.isSuccessfulPasswordResetShown = !state.viewsState.isSuccessfulPasswordResetShown;
    }

    function updateActiveModal(modal: unknown): void {
        // modal could be of VueConstructor type or Object (for composition api components).
        if (state.viewsState.activeModal === modal) {
            state.viewsState.activeModal = null;
            return;
        }
        state.viewsState.activeModal = modal;
    }

    function removeActiveModal(): void {
        state.viewsState.activeModal = null;
    }

    function toggleHasJustLoggenIn(hasJustLoggedIn: boolean | null = null): void {
        if (hasJustLoggedIn === null) {
            state.viewsState.hasJustLoggedIn = !state.viewsState.hasJustLoggedIn;
            return;
        }
        state.viewsState.hasJustLoggedIn = hasJustLoggedIn;
    }

    function changeState(newFetchState: FetchState): void {
        state.viewsState.fetchState = newFetchState;
    }

    function setSatelliteName(satelliteName: string): void {
        state.satelliteName = satelliteName;
    }

    function setPartneredSatellites(partneredSatellites: PartneredSatellite[]): void {
        state.partneredSatellites = partneredSatellites;
    }

    function setSatelliteStatus(isBetaSatellite: boolean): void {
        state.isBetaSatellite = isBetaSatellite;
    }

    function setOnboardingBackRoute(backRoute: string): void {
        state.viewsState.onbAGStepBackRoute = backRoute;
    }

    function setOnboardingAPIKeyStepBackRoute(backRoute: string): void {
        state.viewsState.onbAPIKeyStepBackRoute = backRoute;
    }

    function setOnboardingAPIKey(apiKey: string): void {
        state.viewsState.onbApiKey = apiKey;
    }

    function setOnboardingCleanAPIKey(apiKey: string): void {
        state.viewsState.onbCleanApiKey = apiKey;
    }

    function setCouponCodeBillingUIStatus(couponCodeBillingUIEnabled: boolean): void {
        state.couponCodeBillingUIEnabled = couponCodeBillingUIEnabled;
    }

    function setCouponCodeSignupUIStatus(couponCodeSignupUIEnabled: boolean): void {
        state.couponCodeSignupUIEnabled = couponCodeSignupUIEnabled;
    }

    function setOnboardingOS(os: OnboardingOS): void {
        state.viewsState.onbSelectedOs = os;
    }

    function setPricingPlan(plan: PricingPlanInfo): void {
        state.viewsState.selectedPricingPlan = plan;
    }

    function setManagePassphraseStep(step: ManageProjectPassphraseStep | undefined): void {
        state.viewsState.managePassphraseStep = step;
    }

    function setHasShownPricingPlan(value: boolean): void {
        state.viewsState.hasShownPricingPlan = value;
    }

    function closeDropdowns(): void {
        state.viewsState.activeDropdown = '';
    }

    function clear(): void {
        state.viewsState.activeModal = null;
        state.viewsState.isSuccessfulPasswordResetShown = false;
        state.viewsState.hasJustLoggedIn = false;
        state.viewsState.onbAGStepBackRoute = '';
        state.viewsState.onbAPIKeyStepBackRoute = '';
        state.viewsState.onbCleanApiKey = '';
        state.viewsState.onbApiKey = '';
        state.viewsState.setDefaultPaymentMethodID = '';
        state.viewsState.deletePaymentMethodID = '';
        state.viewsState.onbSelectedOs = null;
        state.viewsState.managePassphraseStep = undefined;
        state.viewsState.selectedPricingPlan = null;
        state.viewsState.hasShownPricingPlan = false;
        state.viewsState.activeDropdown = '';
    }

    return {
        state,
        getConfig,
        toggleActiveDropdown,
        toggleSuccessfulPasswordReset,
        removeActiveModal,
        toggleHasJustLoggenIn,
        changeState,
        setSatelliteName,
        setPartneredSatellites,
        setSatelliteStatus,
        setOnboardingBackRoute,
        setOnboardingAPIKeyStepBackRoute,
        setOnboardingAPIKey,
        setOnboardingCleanAPIKey,
        setCouponCodeBillingUIStatus,
        setCouponCodeSignupUIStatus,
        setOnboardingOS,
        setPricingPlan,
        setManagePassphraseStep,
        setHasShownPricingPlan,
        closeDropdowns,
        updateActiveModal,
        clear,
    };
});
