// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { AppState } from '@/utils/constants/appStateEnum';

import { APP_STATE_MUTATIONS } from '../mutationConstants';

export const appStateModule = {
    state: {
        // Object that contains all states of views
        appState: {
            fetchState: AppState.LOADING,
            isAddTeamMembersPopupShown: false,
            isAccountDropdownShown: false,
            isSelectProjectDropdownShown: false,
            isResourcesDropdownShown: false,
            isSettingsDropdownShown: false,
            isEditProjectDropdownShown: false,
            isFreeCreditsDropdownShown: false,
            isAvailableBalanceDropdownShown: false,
            isPeriodsDropdownShown: false,
            isDeleteProjectPopupShown: false,
            isDeleteAccountPopupShown: false,
            isSortProjectMembersByPopupShown: false,
            isSuccessfulRegistrationShown: false,
            isEditProfilePopupShown: false,
            isChangePasswordPopupShown: false,
            isPaymentSelectionShown: false,
            isCreateProjectButtonShown: false,
            isSaveApiKeyModalShown: false,
        },
        satelliteName: '',
    },
    mutations: {
        // Mutation changing add projectMembers members popup visibility
        [APP_STATE_MUTATIONS.TOGGLE_ADD_TEAMMEMBER_POPUP](state: any): void {
            state.appState.isAddTeamMembersPopupShown = !state.appState.isAddTeamMembersPopupShown;
        },
        // Mutation changing save api key modal visibility
        [APP_STATE_MUTATIONS.TOGGLE_SAVE_API_KEY_MODAL](state: any): void {
            state.appState.isSaveApiKeyModalShown = !state.appState.isSaveApiKeyModalShown;
        },
        // Mutation changing account dropdown visibility
        [APP_STATE_MUTATIONS.TOGGLE_ACCOUNT_DROPDOWN](state: any): void {
            state.appState.isAccountDropdownShown = !state.appState.isAccountDropdownShown;
        },
        // Mutation changing select project dropdown visibility
        [APP_STATE_MUTATIONS.TOGGLE_SELECT_PROJECT_DROPDOWN](state: any): void {
            state.appState.isSelectProjectDropdownShown = !state.appState.isSelectProjectDropdownShown;
        },
        // Mutation changing resources dropdown visibility
        [APP_STATE_MUTATIONS.TOGGLE_RESOURCES_DROPDOWN](state: any): void {
            state.appState.isResourcesDropdownShown = !state.appState.isResourcesDropdownShown;
        },
        // Mutation changing settings dropdown visibility
        [APP_STATE_MUTATIONS.TOGGLE_SETTINGS_DROPDOWN](state: any): void {
            state.appState.isSettingsDropdownShown = !state.appState.isSettingsDropdownShown;
        },
        // Mutation changing edit project dropdown visibility
        [APP_STATE_MUTATIONS.TOGGLE_EDIT_PROJECT_DROPDOWN](state: any): void {
            state.appState.isEditProjectDropdownShown = !state.appState.isEditProjectDropdownShown;
        },
        // Mutation changing free credits dropdown visibility
        [APP_STATE_MUTATIONS.TOGGLE_FREE_CREDITS_DROPDOWN](state: any): void {
            state.appState.isFreeCreditsDropdownShown = !state.appState.isFreeCreditsDropdownShown;
        },
        // Mutation changing available balance dropdown visibility
        [APP_STATE_MUTATIONS.TOGGLE_AVAILABLE_BALANCE_DROPDOWN](state: any): void {
            state.appState.isAvailableBalanceDropdownShown = !state.appState.isAvailableBalanceDropdownShown;
        },
        // Mutation changing periods dropdown visibility
        [APP_STATE_MUTATIONS.TOGGLE_PERIODS_DROPDOWN](state: any): void {
            state.appState.isPeriodsDropdownShown = !state.appState.isPeriodsDropdownShown;
        },
        // Mutation changing delete project popup visibility
        [APP_STATE_MUTATIONS.TOGGLE_DELETE_PROJECT_DROPDOWN](state: any): void {
            state.appState.isDeleteProjectPopupShown = !state.appState.isDeleteProjectPopupShown;
        },
        // Mutation changing delete account popup visibility
        [APP_STATE_MUTATIONS.TOGGLE_DELETE_ACCOUNT_DROPDOWN](state: any): void {
            state.appState.isDeleteAccountPopupShown = !state.appState.isDeleteAccountPopupShown;
        },
        // Mutation changing 'sort project members by' popup visibility.
        [APP_STATE_MUTATIONS.TOGGLE_SORT_PM_BY_DROPDOWN](state: any): void {
            state.appState.isSortProjectMembersByPopupShown = !state.appState.isSortProjectMembersByPopupShown;
        },
        // Mutation changing 'successful registration' area visibility.
        [APP_STATE_MUTATIONS.TOGGLE_SUCCESSFUL_REGISTRATION](state: any): void {
            state.appState.isSuccessfulRegistrationShown = !state.appState.isSuccessfulRegistrationShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_CHANGE_PASSWORD_POPUP](state: any): void {
            state.appState.isChangePasswordPopupShown = !state.appState.isChangePasswordPopupShown;
        },
        [APP_STATE_MUTATIONS.TOGGLE_EDIT_PROFILE_POPUP](state: any): void {
            state.appState.isEditProfilePopupShown = !state.appState.isEditProfilePopupShown;
        },
        [APP_STATE_MUTATIONS.SHOW_SET_DEFAULT_PAYMENT_METHOD_POPUP](state: any, id: string): void {
            state.appState.setDefaultPaymentMethodID = id;
        },
        [APP_STATE_MUTATIONS.SHOW_DELETE_PAYMENT_METHOD_POPUP](state: any, id: string): void {
            state.appState.deletePaymentMethodID = id;
        },
        [APP_STATE_MUTATIONS.SHOW_CREATE_PROJECT_BUTTON](state: any): void {
            state.appState.isCreateProjectButtonShown = true;
        },
        [APP_STATE_MUTATIONS.HIDE_CREATE_PROJECT_BUTTON](state: any): void {
            state.appState.isCreateProjectButtonShown = false;
        },
        // Mutation that closes each popup/dropdown
        [APP_STATE_MUTATIONS.CLOSE_ALL](state: any): void {
            state.appState.isAccountDropdownShown = false;
            state.appState.isSelectProjectDropdownShown = false;
            state.appState.isResourcesDropdownShown = false;
            state.appState.isSettingsDropdownShown = false;
            state.appState.isEditProjectDropdownShown = false;
            state.appState.isFreeCreditsDropdownShown = false;
            state.appState.isAvailableBalanceDropdownShown = false;
            state.appState.isPeriodsDropdownShown = false;
            state.appState.isPaymentSelectionShown = false;
            state.appState.isSortProjectMembersByPopupShown = false;
        },
        [APP_STATE_MUTATIONS.CHANGE_STATE](state: any, newFetchState: AppState): void {
            state.appState.fetchState = newFetchState;
        },
        // Mutation changing payment selection visibility
        [APP_STATE_MUTATIONS.TOGGLE_PAYMENT_SELECTION](state: any, value: boolean): void {
            state.appState.isPaymentSelectionShown = value;
        },
        [APP_STATE_MUTATIONS.SET_NAME](state: any, satelliteName: string): void {
            state.satelliteName = satelliteName;
        },
    },
    actions: {
        // Commits mutation for changing app popups and dropdowns visibility state
        [APP_STATE_ACTIONS.TOGGLE_TEAM_MEMBERS]: function ({commit, state}: any): void {
            if (!state.appState.isAddTeamMembersPopupShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_ADD_TEAMMEMBER_POPUP);
        },
        [APP_STATE_ACTIONS.TOGGLE_SAVE_API_KEY_MODAL]: function ({commit, state}: any): void {
            if (!state.appState.isSaveApiKeyModalShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_SAVE_API_KEY_MODAL);
        },
        [APP_STATE_ACTIONS.TOGGLE_ACCOUNT]: function ({commit, state}: any): void {
            if (!state.appState.isAccountDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_ACCOUNT_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_SELECT_PROJECT_DROPDOWN]: function ({commit, state}: any): void {
            if (!state.appState.isSelectProjectDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_SELECT_PROJECT_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_RESOURCES_DROPDOWN]: function ({commit, state}: any): void {
            if (!state.appState.isResourcesDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_RESOURCES_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_SETTINGS_DROPDOWN]: function ({commit, state}: any): void {
            if (!state.appState.isSettingsDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_SETTINGS_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_EDIT_PROJECT_DROPDOWN]: function ({commit, state}: any): void {
            if (!state.appState.isEditProjectDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_EDIT_PROJECT_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_FREE_CREDITS_DROPDOWN]: function ({commit, state}: any): void {
            if (!state.appState.isFreeCreditsDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_FREE_CREDITS_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_AVAILABLE_BALANCE_DROPDOWN]: function ({commit, state}: any): void {
            if (!state.appState.isAvailableBalanceDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_AVAILABLE_BALANCE_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_PERIODS_DROPDOWN]: function ({commit, state}: any): void {
            if (!state.appState.isPeriodsDropdownShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_PERIODS_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_PAYMENT_SELECTION]: function ({commit, state}: any, value: boolean): void {
            if (!state.appState.isPaymentSelectionShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_PAYMENT_SELECTION, value);
        },
        [APP_STATE_ACTIONS.TOGGLE_DEL_PROJ]: function ({commit, state}: any): void {
            if (!state.appState.isDeleteProjectPopupShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_DELETE_PROJECT_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_DEL_ACCOUNT]: function ({commit, state}: any): void {
            if (!state.appState.isDeleteAccountPopupShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_DELETE_ACCOUNT_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_SORT_PM_BY_DROPDOWN]: function ({commit, state}: any): void {
            if (!state.appState.isSortProjectMembersByPopupShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_SORT_PM_BY_DROPDOWN);
        },
        [APP_STATE_ACTIONS.TOGGLE_SUCCESSFUL_REGISTRATION]: function ({commit, state}: any): void {
            if (!state.appState.isSuccessfulRegistrationShown) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.TOGGLE_SUCCESSFUL_REGISTRATION);
        },
        [APP_STATE_ACTIONS.TOGGLE_CHANGE_PASSWORD_POPUP]: function ({commit}: any): void {
            commit(APP_STATE_MUTATIONS.TOGGLE_CHANGE_PASSWORD_POPUP);
        },
        [APP_STATE_ACTIONS.TOGGLE_EDIT_PROFILE_POPUP]: function ({commit}: any): void {
            commit(APP_STATE_MUTATIONS.TOGGLE_EDIT_PROFILE_POPUP);
        },
        [APP_STATE_ACTIONS.SHOW_SET_DEFAULT_PAYMENT_METHOD_POPUP]: function ({commit, state}: any, methodID: string): void {
            if (!state.appState.setDefaultPaymentMethodID) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.SHOW_SET_DEFAULT_PAYMENT_METHOD_POPUP, methodID);
        },
        [APP_STATE_ACTIONS.SHOW_DELETE_PAYMENT_METHOD_POPUP]: function ({commit, state}: any, methodID: string): void {
            if (!state.appState.deletePaymentMethodID) {
                commit(APP_STATE_MUTATIONS.CLOSE_ALL);
            }

            commit(APP_STATE_MUTATIONS.SHOW_DELETE_PAYMENT_METHOD_POPUP, methodID);
        },
        [APP_STATE_ACTIONS.SHOW_CREATE_PROJECT_BUTTON]: function ({commit}: any): void {
            commit(APP_STATE_MUTATIONS.SHOW_CREATE_PROJECT_BUTTON);
        },
        [APP_STATE_ACTIONS.HIDE_CREATE_PROJECT_BUTTON]: function ({commit}: any): void {
            commit(APP_STATE_MUTATIONS.HIDE_CREATE_PROJECT_BUTTON);
        },
        [APP_STATE_ACTIONS.CLOSE_POPUPS]: function ({commit}: any): void {
            commit(APP_STATE_MUTATIONS.CLOSE_ALL);
        },
        [APP_STATE_ACTIONS.CHANGE_STATE]: function ({commit}: any, newFetchState: AppState): void {
            commit(APP_STATE_MUTATIONS.CHANGE_STATE, newFetchState);
        },
        [APP_STATE_ACTIONS.SET_SATELLITE_NAME]: function ({commit}: any, satelliteName: string): void {
            commit(APP_STATE_MUTATIONS.SET_NAME, satelliteName);
        },
    },
};
