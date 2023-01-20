// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { OnboardingOS, PartneredSatellite, PricingPlanInfo } from '@/types/common';
import { AppState } from '@/utils/constants/appStateEnum';
import { ManageProjectPassphraseStep } from '@/types/managePassphrase';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { MetaUtils } from '@/utils/meta';

// Object that contains all states of views
class ViewsState {
    constructor(
        public fetchState = AppState.LOADING,
        public isSuccessfulPasswordResetShown = false,
        public isBillingNotificationShown = true,
        public hasJustLoggedIn = false,
        public onbAGStepBackRoute = '',
        public onbAPIKeyStepBackRoute = '',
        public onbCleanApiKey = '',
        public onbApiKey = '',
        public setDefaultPaymentMethodID = '',
        public deletePaymentMethodID = '',
        public onbSelectedOs: OnboardingOS | null = null,
        public selectedPricingPlan: PricingPlanInfo | null = null,
        public managePassphraseStep: ManageProjectPassphraseStep | undefined = undefined,
        public activeDropdown = 'none',
        // activeModal could be of VueConstructor type or Object (for composition api components).
        public activeModal: unknown | null = null,
    ) {}
}

class State {
    constructor(
        public appState: ViewsState = new ViewsState(),
        public satelliteName = '',
        public partneredSatellites = new Array<PartneredSatellite>(),
        public isBetaSatellite = false,
        public couponCodeBillingUIEnabled = false,
        public couponCodeSignupUIEnabled = false,
        public isNewProjectDashboard = false,
        public isAllProjectsDashboard = MetaUtils.getMetaContent('all-projects-dashboard') === 'true',
        public isNewAccessGrantFlow = false,
    ) {}
}

interface AppContext {
    state: State
    commit: (string, ...unknown) => void
}

export const appStateModule = {
    state: new State(),
    mutations: {
        [APP_STATE_MUTATIONS.TOGGLE_SUCCESSFUL_PASSWORD_RESET](state: State): void {
            state.appState.isSuccessfulPasswordResetShown = !state.appState.isSuccessfulPasswordResetShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_HAS_JUST_LOGGED_IN](state: State): void {
            state.appState.hasJustLoggedIn = !state.appState.hasJustLoggedIn;
        },
        [APP_STATE_MUTATIONS.CLOSE_BILLING_NOTIFICATION](state: State): void {
            state.appState.isBillingNotificationShown = false;
        },
        [APP_STATE_MUTATIONS.CLEAR](state: State): void {
            state.appState.activeModal = null;
            state.appState.isSuccessfulPasswordResetShown = false;
            state.appState.hasJustLoggedIn = false;
            state.appState.onbAGStepBackRoute = '';
            state.appState.onbAPIKeyStepBackRoute = '';
            state.appState.onbCleanApiKey = '';
            state.appState.onbApiKey = '';
            state.appState.setDefaultPaymentMethodID = '';
            state.appState.deletePaymentMethodID = '';
            state.appState.onbSelectedOs = null;
            state.appState.managePassphraseStep = undefined;
            state.appState.selectedPricingPlan = null;
        },
        [APP_STATE_MUTATIONS.CHANGE_STATE](state: State, newFetchState: AppState): void {
            state.appState.fetchState = newFetchState;
        },
        [APP_STATE_MUTATIONS.SET_SATELLITE_NAME](state: State, satelliteName: string): void {
            state.satelliteName = satelliteName;
        },
        [APP_STATE_MUTATIONS.SET_PARTNERED_SATELLITES](state: State, partneredSatellites: PartneredSatellite[]): void {
            state.partneredSatellites = partneredSatellites;
        },
        [APP_STATE_MUTATIONS.SET_SATELLITE_STATUS](state: State, isBetaSatellite: boolean): void {
            state.isBetaSatellite = isBetaSatellite;
        },
        [APP_STATE_MUTATIONS.SET_PROJECT_DASHBOARD_STATUS](state: State, isNewProjectDashboard: boolean): void {
            state.isNewProjectDashboard = isNewProjectDashboard;
        },
        [APP_STATE_MUTATIONS.SET_ACCESS_GRANT_FLOW_STATUS](state: State, isNewAccessGrantFlow: boolean): void {
            state.isNewAccessGrantFlow = isNewAccessGrantFlow;
        },
        [APP_STATE_MUTATIONS.SET_ONB_AG_NAME_STEP_BACK_ROUTE](state: State, backRoute: string): void {
            state.appState.onbAGStepBackRoute = backRoute;
        },
        [APP_STATE_MUTATIONS.SET_ONB_API_KEY_STEP_BACK_ROUTE](state: State, backRoute: string): void {
            state.appState.onbAPIKeyStepBackRoute = backRoute;
        },
        [APP_STATE_MUTATIONS.SET_ONB_API_KEY](state: State, apiKey: string): void {
            state.appState.onbApiKey = apiKey;
        },
        [APP_STATE_MUTATIONS.SET_ONB_CLEAN_API_KEY](state: State, apiKey: string): void {
            state.appState.onbCleanApiKey = apiKey;
        },
        [APP_STATE_MUTATIONS.SET_COUPON_CODE_BILLING_UI_STATUS](state: State, couponCodeBillingUIEnabled: boolean): void {
            state.couponCodeBillingUIEnabled = couponCodeBillingUIEnabled;
        },
        [APP_STATE_MUTATIONS.SET_COUPON_CODE_SIGNUP_UI_STATUS](state: State, couponCodeSignupUIEnabled: boolean): void {
            state.couponCodeSignupUIEnabled = couponCodeSignupUIEnabled;
        },
        [APP_STATE_MUTATIONS.SET_ONB_OS](state: State, os: OnboardingOS): void {
            state.appState.onbSelectedOs = os;
        },
        [APP_STATE_MUTATIONS.SET_PRICING_PLAN](state: State, plan: PricingPlanInfo): void {
            state.appState.selectedPricingPlan = plan;
        },
        [APP_STATE_MUTATIONS.SET_MANAGE_PASSPHRASE_STEP](state: State, step: ManageProjectPassphraseStep | undefined): void {
            state.appState.managePassphraseStep = step;
        },
        [APP_STATE_MUTATIONS.TOGGLE_ACTIVE_DROPDOWN](state: State, dropdown: string): void {
            state.appState.activeDropdown = dropdown;
        },
        [APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL](state: State, modal: unknown): void {
            // modal could be of VueConstructor type or Object (for composition api components).
            if (state.appState.activeModal === modal) {
                state.appState.activeModal = null;
                return;
            }
            state.appState.activeModal = modal;
        },
        [APP_STATE_MUTATIONS.REMOVE_ACTIVE_MODAL](state: State): void {
            state.appState.activeModal = null;
        },
    },
    actions: {
        [APP_STATE_ACTIONS.TOGGLE_ACTIVE_DROPDOWN]: function ({ commit, state }: AppContext, dropdown: string): void {
            if (state.appState.activeDropdown === dropdown) {
                commit(APP_STATE_MUTATIONS.TOGGLE_ACTIVE_DROPDOWN, 'none');
                return;
            }
            commit(APP_STATE_MUTATIONS.TOGGLE_ACTIVE_DROPDOWN, dropdown);
        },
        [APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_PASSWORD_RESET]: function ({ commit, state }: AppContext): void {
            if (!state.appState.isSuccessfulPasswordResetShown) {
                commit(APP_STATE_MUTATIONS.TOGGLE_ACTIVE_DROPDOWN, 'none');
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_SUCCESSFUL_PASSWORD_RESET);
        },
        [APP_STATE_ACTIONS.CLOSE_POPUPS]: function ({ commit }: AppContext): void {
            commit(APP_STATE_MUTATIONS.TOGGLE_ACTIVE_DROPDOWN, '');
        },
        [APP_STATE_ACTIONS.CLEAR]: function ({ commit }: AppContext): void {
            commit(APP_STATE_MUTATIONS.CLEAR);
            commit(APP_STATE_MUTATIONS.TOGGLE_ACTIVE_DROPDOWN, '');
        },
        [APP_STATE_ACTIONS.CHANGE_STATE]: function ({ commit }: AppContext, newFetchState: AppState): void {
            commit(APP_STATE_MUTATIONS.CHANGE_STATE, newFetchState);
        },
        [APP_STATE_ACTIONS.SET_SATELLITE_NAME]: function ({ commit }: AppContext, satelliteName: string): void {
            commit(APP_STATE_MUTATIONS.SET_SATELLITE_NAME, satelliteName);
        },
        [APP_STATE_ACTIONS.SET_PARTNERED_SATELLITES]: function ({ commit }: AppContext, partneredSatellites: PartneredSatellite[]): void {
            commit(APP_STATE_MUTATIONS.SET_PARTNERED_SATELLITES, partneredSatellites);
        },
        [APP_STATE_ACTIONS.SET_SATELLITE_STATUS]: function ({ commit }: AppContext, isBetaSatellite: boolean): void {
            commit(APP_STATE_MUTATIONS.SET_SATELLITE_STATUS, isBetaSatellite);
        },
        [APP_STATE_ACTIONS.SET_PROJECT_DASHBOARD_STATUS]: function ({ commit }: AppContext, isNewProjectDashboard: boolean): void {
            commit(APP_STATE_MUTATIONS.SET_PROJECT_DASHBOARD_STATUS, isNewProjectDashboard);
        },
        [APP_STATE_ACTIONS.SET_COUPON_CODE_BILLING_UI_STATUS]: function ({ commit }: AppContext, couponCodeBillingUIEnabled: boolean): void {
            commit(APP_STATE_MUTATIONS.SET_COUPON_CODE_BILLING_UI_STATUS, couponCodeBillingUIEnabled);
        },
        [APP_STATE_ACTIONS.SET_COUPON_CODE_SIGNUP_UI_STATUS]: function ({ commit }: AppContext, couponCodeSignupUIEnabled: boolean): void {
            commit(APP_STATE_MUTATIONS.SET_COUPON_CODE_SIGNUP_UI_STATUS, couponCodeSignupUIEnabled);
        },
    },
};
