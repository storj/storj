// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { OnboardingOS, PartneredSatellite, PricingPlanInfo } from '@/types/common';
import { FetchState } from '@/utils/constants/fetchStateEnum';
import { ManageProjectPassphraseStep } from '@/types/managePassphrase';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { MetaUtils } from '@/utils/meta';
import { FrontendConfig, FrontendConfigApi } from '@/types/config';
import { StoreModule } from '@/types/store';

// Object that contains all states of views
class ViewsState {
    constructor(
        public fetchState = FetchState.LOADING,
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
        // this field is mainly used on the all projects dashboard as an exit condition
        // for when the dashboard opens the pricing plan and the pricing plan navigates back repeatedly.
        public hasShownPricingPlan = false,
    ) {}
}

export class AppState {
    constructor(
        public viewsState: ViewsState = new ViewsState(),
        public satelliteName = '',
        public partneredSatellites = new Array<PartneredSatellite>(),
        public isBetaSatellite = false,
        public couponCodeBillingUIEnabled = false,
        public couponCodeSignupUIEnabled = false,
        public isAllProjectsDashboard = MetaUtils.getMetaContent('all-projects-dashboard') === 'true',
        public config: FrontendConfig = new FrontendConfig(),
    ) {}
}

interface AppContext {
    state: AppState
    commit: (string, ...unknown) => void
}

export function makeAppStateModule(configApi: FrontendConfigApi): StoreModule<AppState, AppContext> {
    return {
        state: new AppState(),
        mutations: {
            [APP_STATE_MUTATIONS.TOGGLE_SUCCESSFUL_PASSWORD_RESET](state: AppState): void {
                state.viewsState.isSuccessfulPasswordResetShown = !state.viewsState.isSuccessfulPasswordResetShown;
            },
            [APP_STATE_MUTATIONS.TOGGLE_HAS_JUST_LOGGED_IN](state: AppState, hasJustLoggedIn: boolean | null = null): void {
                if (hasJustLoggedIn === null) {
                    state.viewsState.hasJustLoggedIn = !state.viewsState.hasJustLoggedIn;
                    return;
                }
                state.viewsState.hasJustLoggedIn = hasJustLoggedIn;
            },
            [APP_STATE_MUTATIONS.CLOSE_BILLING_NOTIFICATION](state: AppState): void {
                state.viewsState.isBillingNotificationShown = false;
            },
            [APP_STATE_MUTATIONS.CLEAR](state: AppState): void {
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
            },
            [APP_STATE_MUTATIONS.CHANGE_FETCH_STATE](state: AppState, newFetchState: FetchState): void {
                state.viewsState.fetchState = newFetchState;
            },
            [APP_STATE_MUTATIONS.SET_SATELLITE_NAME](state: AppState, satelliteName: string): void {
                state.satelliteName = satelliteName;
            },
            [APP_STATE_MUTATIONS.SET_PARTNERED_SATELLITES](state: AppState, partneredSatellites: PartneredSatellite[]): void {
                state.partneredSatellites = partneredSatellites;
            },
            [APP_STATE_MUTATIONS.SET_SATELLITE_STATUS](state: AppState, isBetaSatellite: boolean): void {
                state.isBetaSatellite = isBetaSatellite;
            },
            [APP_STATE_MUTATIONS.SET_ONB_AG_NAME_STEP_BACK_ROUTE](state: AppState, backRoute: string): void {
                state.viewsState.onbAGStepBackRoute = backRoute;
            },
            [APP_STATE_MUTATIONS.SET_ONB_API_KEY_STEP_BACK_ROUTE](state: AppState, backRoute: string): void {
                state.viewsState.onbAPIKeyStepBackRoute = backRoute;
            },
            [APP_STATE_MUTATIONS.SET_ONB_API_KEY](state: AppState, apiKey: string): void {
                state.viewsState.onbApiKey = apiKey;
            },
            [APP_STATE_MUTATIONS.SET_ONB_CLEAN_API_KEY](state: AppState, apiKey: string): void {
                state.viewsState.onbCleanApiKey = apiKey;
            },
            [APP_STATE_MUTATIONS.SET_COUPON_CODE_BILLING_UI_STATUS](state: AppState, couponCodeBillingUIEnabled: boolean): void {
                state.couponCodeBillingUIEnabled = couponCodeBillingUIEnabled;
            },
            [APP_STATE_MUTATIONS.SET_COUPON_CODE_SIGNUP_UI_STATUS](state: AppState, couponCodeSignupUIEnabled: boolean): void {
                state.couponCodeSignupUIEnabled = couponCodeSignupUIEnabled;
            },
            [APP_STATE_MUTATIONS.SET_ONB_OS](state: AppState, os: OnboardingOS): void {
                state.viewsState.onbSelectedOs = os;
            },
            [APP_STATE_MUTATIONS.SET_PRICING_PLAN](state: AppState, plan: PricingPlanInfo): void {
                state.viewsState.selectedPricingPlan = plan;
            },
            [APP_STATE_MUTATIONS.SET_MANAGE_PASSPHRASE_STEP](state: AppState, step: ManageProjectPassphraseStep | undefined): void {
                state.viewsState.managePassphraseStep = step;
            },
            [APP_STATE_MUTATIONS.SET_CONFIG](state: AppState, config: FrontendConfig): void {
                state.config = config;
            },
            [APP_STATE_MUTATIONS.TOGGLE_ACTIVE_DROPDOWN](state: AppState, dropdown: string): void {
                state.viewsState.activeDropdown = dropdown;
            },
            [APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL](state: AppState, modal: unknown): void {
                // modal could be of VueConstructor type or Object (for composition api components).
                if (state.viewsState.activeModal === modal) {
                    state.viewsState.activeModal = null;
                    return;
                }
                state.viewsState.activeModal = modal;
            },
            [APP_STATE_MUTATIONS.REMOVE_ACTIVE_MODAL](state: AppState): void {
                state.viewsState.activeModal = null;
            },
            [APP_STATE_MUTATIONS.SET_HAS_SHOWN_PRICING_PLAN](state: AppState, value: boolean): void {
                state.viewsState.hasShownPricingPlan = value;
            },
        },
        actions: {
            [APP_STATE_ACTIONS.TOGGLE_ACTIVE_DROPDOWN]: function ({ commit, state }: AppContext, dropdown: string): void {
                if (state.viewsState.activeDropdown === dropdown) {
                    commit(APP_STATE_MUTATIONS.TOGGLE_ACTIVE_DROPDOWN, 'none');
                    return;
                }
                commit(APP_STATE_MUTATIONS.TOGGLE_ACTIVE_DROPDOWN, dropdown);
            },
            [APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_PASSWORD_RESET]: function ({ commit, state }: AppContext): void {
                if (!state.viewsState.isSuccessfulPasswordResetShown) {
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
            [APP_STATE_ACTIONS.CHANGE_FETCH_STATE]: function ({ commit }: AppContext, newFetchState: FetchState): void {
                commit(APP_STATE_MUTATIONS.CHANGE_FETCH_STATE, newFetchState);
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
            [APP_STATE_ACTIONS.SET_COUPON_CODE_BILLING_UI_STATUS]: function ({ commit }: AppContext, couponCodeBillingUIEnabled: boolean): void {
                commit(APP_STATE_MUTATIONS.SET_COUPON_CODE_BILLING_UI_STATUS, couponCodeBillingUIEnabled);
            },
            [APP_STATE_ACTIONS.SET_COUPON_CODE_SIGNUP_UI_STATUS]: function ({ commit }: AppContext, couponCodeSignupUIEnabled: boolean): void {
                commit(APP_STATE_MUTATIONS.SET_COUPON_CODE_SIGNUP_UI_STATUS, couponCodeSignupUIEnabled);
            },
            [APP_STATE_ACTIONS.FETCH_CONFIG]: async function ({ commit }: AppContext): Promise<FrontendConfig> {
                const result = await configApi.get();
                commit(APP_STATE_MUTATIONS.SET_CONFIG, result);
                return result;
            },
        },
    };
}
