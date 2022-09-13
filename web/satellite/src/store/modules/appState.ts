// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { OnboardingOS, PartneredSatellite } from '@/types/common';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { AppState } from '@/utils/constants/appStateEnum';

// Object that contains all states of views
class ViewsState {
    constructor(
        public fetchState = AppState.LOADING,
        public isAddTeamMembersModalShown = false,
        public isAccountDropdownShown = false,
        public isSelectProjectDropdownShown = false,
        public isResourcesDropdownShown = false,
        public isQuickStartDropdownShown = false,
        public isSettingsDropdownShown = false,
        public isEditProjectDropdownShown = false,
        public isFreeCreditsDropdownShown = false,
        public isAvailableBalanceDropdownShown = false,
        public isPeriodsDropdownShown = false,
        public isBucketNamesDropdownShown = false,
        public isAGDatePickerShown = false,
        public isChartsDatePickerShown = false,
        public isPermissionsDropdownShown = false,
        public isEditProfileModalShown = false,
        public isChangePasswordModalShown = false,
        public isPaymentSelectionShown = false,
        public isUploadCancelPopupVisible = false,
        public isSuccessfulPasswordResetShown = false,
        public isCreateProjectPromptModalShown = false,
        public isCreateProjectModalShown = false,
        public isAddPMModalShown = false,
        public isOpenBucketModalShown = false,
        public isMFARecoveryModalShown = false,
        public isEnableMFAModalShown = false,
        public isDisableMFAModalShown = false,
        public isAddTokenFundsModalShown = false,
        public isShareBucketModalShown = false,
        public isBillingNotificationShown = true,

        public onbAGStepBackRoute = '',
        public onbAPIKeyStepBackRoute = '',
        public onbCleanApiKey = '',
        public onbApiKey = '',
        public setDefaultPaymentMethodID = '',
        public deletePaymentMethodID = '',
        public onbSelectedOs: OnboardingOS | null = null,
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
        public isNewObjectsFlow = false,
    ){}
}

interface AppContext {
    state: State
    commit: (string, ...unknown) => void
}

export const appStateModule = {
    state: new State(),
    mutations: {
        [APP_STATE_MUTATIONS.TOGGLE_ADD_TEAM_MEMBERS_MODAL](state: State): void {
            state.appState.isAddTeamMembersModalShown = !state.appState.isAddTeamMembersModalShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_ACCOUNT_DROPDOWN](state: State): void {
            state.appState.isAccountDropdownShown = !state.appState.isAccountDropdownShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_SELECT_PROJECT_DROPDOWN](state: State): void {
            state.appState.isSelectProjectDropdownShown = !state.appState.isSelectProjectDropdownShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_RESOURCES_DROPDOWN](state: State): void {
            state.appState.isResourcesDropdownShown = !state.appState.isResourcesDropdownShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_QUICK_START_DROPDOWN](state: State): void {
            state.appState.isQuickStartDropdownShown = !state.appState.isQuickStartDropdownShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_SETTINGS_DROPDOWN](state: State): void {
            state.appState.isSettingsDropdownShown = !state.appState.isSettingsDropdownShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_EDIT_PROJECT_DROPDOWN](state: State): void {
            state.appState.isEditProjectDropdownShown = !state.appState.isEditProjectDropdownShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_FREE_CREDITS_DROPDOWN](state: State): void {
            state.appState.isFreeCreditsDropdownShown = !state.appState.isFreeCreditsDropdownShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_AVAILABLE_BALANCE_DROPDOWN](state: State): void {
            state.appState.isAvailableBalanceDropdownShown = !state.appState.isAvailableBalanceDropdownShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_PERIODS_DROPDOWN](state: State): void {
            state.appState.isPeriodsDropdownShown = !state.appState.isPeriodsDropdownShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_AG_DATEPICKER_DROPDOWN](state: State): void {
            state.appState.isAGDatePickerShown = !state.appState.isAGDatePickerShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_CHARTS_DATEPICKER_DROPDOWN](state: State): void {
            state.appState.isChartsDatePickerShown = !state.appState.isChartsDatePickerShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_BUCKET_NAMES_DROPDOWN](state: State): void {
            state.appState.isBucketNamesDropdownShown = !state.appState.isBucketNamesDropdownShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_PERMISSIONS_DROPDOWN](state: State): void {
            state.appState.isPermissionsDropdownShown = !state.appState.isPermissionsDropdownShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_SUCCESSFUL_PASSWORD_RESET](state: State): void {
            state.appState.isSuccessfulPasswordResetShown = !state.appState.isSuccessfulPasswordResetShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_CHANGE_PASSWORD_MODAL_SHOWN](state: State): void {
            state.appState.isChangePasswordModalShown = !state.appState.isChangePasswordModalShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_EDIT_PROFILE_MODAL_SHOWN](state: State): void {
            state.appState.isEditProfileModalShown = !state.appState.isEditProfileModalShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_UPLOAD_CANCEL_POPUP](state: State): void {
            state.appState.isUploadCancelPopupVisible = !state.appState.isUploadCancelPopupVisible;
        },
        [APP_STATE_MUTATIONS.TOGGLE_CREATE_PROJECT_PROMPT_POPUP](state: State): void {
            state.appState.isCreateProjectPromptModalShown = !state.appState.isCreateProjectPromptModalShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_CREATE_PROJECT_POPUP](state: State): void {
            state.appState.isCreateProjectModalShown = !state.appState.isCreateProjectModalShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_IS_ADD_PM_MODAL_SHOWN](state: State): void {
            state.appState.isAddPMModalShown = !state.appState.isAddPMModalShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_OPEN_BUCKET_MODAL_SHOWN](state: State): void {
            state.appState.isOpenBucketModalShown = !state.appState.isOpenBucketModalShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_MFA_RECOVERY_MODAL_SHOWN](state: State): void {
            state.appState.isMFARecoveryModalShown = !state.appState.isMFARecoveryModalShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_ENABLE_MFA_MODAL_SHOWN](state: State): void {
            state.appState.isEnableMFAModalShown = !state.appState.isEnableMFAModalShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_DISABLE_MFA_MODAL_SHOWN](state: State): void {
            state.appState.isDisableMFAModalShown = !state.appState.isDisableMFAModalShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_ADD_TOKEN_FUNDS_MODAL_SHOWN](state: State): void {
            state.appState.isAddTokenFundsModalShown = !state.appState.isAddTokenFundsModalShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_SHARE_BUCKET_MODAL_SHOWN](state: State): void {
            state.appState.isShareBucketModalShown = !state.appState.isShareBucketModalShown;
        },
        [APP_STATE_MUTATIONS.SHOW_SET_DEFAULT_PAYMENT_METHOD_POPUP](state: State, id: string): void {
            state.appState.setDefaultPaymentMethodID = id;
        },
        [APP_STATE_MUTATIONS.SHOW_DELETE_PAYMENT_METHOD_POPUP](state: State, id: string): void {
            state.appState.deletePaymentMethodID = id;
        },
        [APP_STATE_MUTATIONS.CLOSE_BILLING_NOTIFICATION](state: State): void {
            state.appState.isBillingNotificationShown = false;
        },
        [APP_STATE_MUTATIONS.CLOSE_ALL](state: State): void {
            state.appState.isAccountDropdownShown = false;
            state.appState.isSelectProjectDropdownShown = false;
            state.appState.isResourcesDropdownShown = false;
            state.appState.isQuickStartDropdownShown = false;
            state.appState.isSettingsDropdownShown = false;
            state.appState.isEditProjectDropdownShown = false;
            state.appState.isFreeCreditsDropdownShown = false;
            state.appState.isAvailableBalanceDropdownShown = false;
            state.appState.isPermissionsDropdownShown = false;
            state.appState.isPeriodsDropdownShown = false;
            state.appState.isPaymentSelectionShown = false;
            state.appState.isAGDatePickerShown = false;
            state.appState.isChartsDatePickerShown = false;
            state.appState.isBucketNamesDropdownShown = false;
        },
        [APP_STATE_MUTATIONS.CHANGE_STATE](state: State, newFetchState: AppState): void {
            state.appState.fetchState = newFetchState;
        },
        [APP_STATE_MUTATIONS.TOGGLE_PAYMENT_SELECTION](state: State, value: boolean): void {
            state.appState.isPaymentSelectionShown = value;
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
        [APP_STATE_MUTATIONS.SET_OBJECTS_FLOW_STATUS](state: State, isNewObjectsFlow: boolean): void {
            state.isNewObjectsFlow = isNewObjectsFlow;
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
    },
    actions: {
        [APP_STATE_ACTIONS.TOGGLE_ACCOUNT]: function ({ commit, state }: AppContext): void {
            if (!state.appState.isAccountDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_ACCOUNT_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_SELECT_PROJECT_DROPDOWN]: function ({ commit, state }: AppContext): void {
            if (!state.appState.isSelectProjectDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_SELECT_PROJECT_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_RESOURCES_DROPDOWN]: function ({ commit, state }: AppContext): void {
            if (!state.appState.isResourcesDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_RESOURCES_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_QUICK_START_DROPDOWN]: function ({ commit, state }: AppContext): void {
            if (!state.appState.isQuickStartDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_QUICK_START_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_SETTINGS_DROPDOWN]: function ({ commit, state }: AppContext): void {
            if (!state.appState.isSettingsDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_SETTINGS_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_EDIT_PROJECT_DROPDOWN]: function ({ commit, state }: AppContext): void {
            if (!state.appState.isEditProjectDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_EDIT_PROJECT_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_FREE_CREDITS_DROPDOWN]: function ({ commit, state }: AppContext): void {
            if (!state.appState.isFreeCreditsDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_FREE_CREDITS_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_AVAILABLE_BALANCE_DROPDOWN]: function ({ commit, state }: AppContext): void {
            if (!state.appState.isAvailableBalanceDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_AVAILABLE_BALANCE_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_PERIODS_DROPDOWN]: function ({ commit, state }: AppContext): void {
            if (!state.appState.isPeriodsDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_PERIODS_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_AG_DATEPICKER_DROPDOWN]: function ({ commit, state }: AppContext): void {
            if (!state.appState.isAGDatePickerShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_AG_DATEPICKER_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_CHARTS_DATEPICKER_DROPDOWN]: function ({ commit, state }: AppContext): void {
            if (!state.appState.isChartsDatePickerShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_CHARTS_DATEPICKER_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_BUCKET_NAMES_DROPDOWN]: function ({ commit, state }: AppContext): void {
            if (!state.appState.isBucketNamesDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_BUCKET_NAMES_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_PERMISSIONS_DROPDOWN]: function ({ commit, state }: AppContext): void {
            if (!state.appState.isPermissionsDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_PERMISSIONS_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_PAYMENT_SELECTION]: function ({ commit, state }: AppContext, value: boolean): void {
            if (!state.appState.isPaymentSelectionShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_PAYMENT_SELECTION, value);
        },
        [APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_PASSWORD_RESET]: function ({ commit, state }: AppContext): void {
            if (!state.appState.isSuccessfulPasswordResetShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_SUCCESSFUL_PASSWORD_RESET);
        },
        [APP_STATE_ACTIONS.TOGGLE_UPLOAD_CANCEL_POPUP]: function ({ commit }: AppContext): void {
            commit(APP_STATE_MUTATIONS.TOGGLE_UPLOAD_CANCEL_POPUP);
        },
        [APP_STATE_ACTIONS.SHOW_SET_DEFAULT_PAYMENT_METHOD_POPUP]: function ({ commit, state }: AppContext, methodID: string): void {
            if (!state.appState.setDefaultPaymentMethodID) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.SHOW_SET_DEFAULT_PAYMENT_METHOD_POPUP, methodID);
        },
        [APP_STATE_ACTIONS.SHOW_DELETE_PAYMENT_METHOD_POPUP]: function ({ commit, state }: AppContext, methodID: string): void {
            if (!state.appState.deletePaymentMethodID) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.SHOW_DELETE_PAYMENT_METHOD_POPUP, methodID);
        },
        [APP_STATE_ACTIONS.CLOSE_POPUPS]: function ({ commit }: AppContext): void {
            commit(APP_STATE_MUTATIONS.CLOSE_ALL);
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
        [APP_STATE_ACTIONS.SET_OBJECTS_FLOW_STATUS]: function ({ commit }: AppContext, isNewObjectsFlow: boolean): void {
            commit(APP_STATE_MUTATIONS.SET_OBJECTS_FLOW_STATUS, isNewObjectsFlow);
        },
        [APP_STATE_ACTIONS.SET_COUPON_CODE_BILLING_UI_STATUS]: function ({ commit }: AppContext, couponCodeBillingUIEnabled: boolean): void {
            commit(APP_STATE_MUTATIONS.SET_COUPON_CODE_BILLING_UI_STATUS, couponCodeBillingUIEnabled);
        },
        [APP_STATE_ACTIONS.SET_COUPON_CODE_SIGNUP_UI_STATUS]: function ({ commit }: AppContext, couponCodeSignupUIEnabled: boolean): void {
            commit(APP_STATE_MUTATIONS.SET_COUPON_CODE_SIGNUP_UI_STATUS, couponCodeSignupUIEnabled);
        },
    },
};
