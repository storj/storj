// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export const APP_STATE_ACTIONS = {
    TOGGLE_TEAM_MEMBERS: 'toggleAddTeamMembersPopup',
    TOGGLE_SAVE_API_KEY_MODAL: 'toggleSaveApiKeyModal',
    TOGGLE_ACCOUNT: 'toggleAccountDropdown',
    TOGGLE_SELECT_PROJECT_DROPDOWN: 'toggleSelectProjectDropdown',
    TOGGLE_RESOURCES_DROPDOWN: 'toggleResourcesDropdown',
    TOGGLE_SETTINGS_DROPDOWN: 'toggleSettingsDropdown',
    TOGGLE_EDIT_PROJECT_DROPDOWN: 'toggleEditProjectDropdown',
    TOGGLE_FREE_CREDITS_DROPDOWN: 'toggleFreeCreditsDropdown',
    TOGGLE_AVAILABLE_BALANCE_DROPDOWN: 'toggleAvailableBalanceDropdown',
    TOGGLE_PERIODS_DROPDOWN: 'togglePeriodsDropdown',
    TOGGLE_DEL_PROJ: 'toggleDeleteProjectPopup',
    TOGGLE_DEL_ACCOUNT: 'toggleDeleteAccountPopup',
    TOGGLE_SORT_PM_BY_DROPDOWN: 'toggleSortProjectMembersByPopup',
    TOGGLE_SUCCESSFUL_REGISTRATION: 'TOGGLE_SUCCESSFUL_REGISTRATION',
    TOGGLE_SUCCESSFUL_PROJECT_CREATION_POPUP: 'toggleSuccessfulProjectCreationPopup',
    TOGGLE_EDIT_PROFILE_POPUP: 'toggleEditProfilePopup',
    TOGGLE_CHANGE_PASSWORD_POPUP: 'toggleChangePasswordPopup',
    SHOW_SET_DEFAULT_PAYMENT_METHOD_POPUP: 'showSetDefaultPaymentMethodPopup',
    CLOSE_SET_DEFAULT_PAYMENT_METHOD_POPUP: 'closeSetDefaultPaymentMethodPopup',
    SHOW_DELETE_PAYMENT_METHOD_POPUP: 'showDeletePaymentMethodPopup',
    CLOSE_DELETE_PAYMENT_METHOD_POPUP: 'closeDeletePaymentMethodPopup',
    CLOSE_POPUPS: 'closePopups',
    CHANGE_STATE: 'changeFetchState',
    TOGGLE_PAYMENT_SELECTION: 'TOGGLE_PAYMENT_SELECTION',
    SET_SATELLITE_NAME: 'SET_SATELLITE_NAME',
    SHOW_CREATE_PROJECT_BUTTON: 'SHOW_CREATE_PROJECT_BUTTON',
    HIDE_CREATE_PROJECT_BUTTON: 'HIDE_CREATE_PROJECT_BUTTON',
};

export const NOTIFICATION_ACTIONS = {
    SUCCESS: 'success',
    ERROR: 'error',
    NOTIFY: 'notify',
    WARNING: 'WARNING',
    DELETE: 'deleteNotification',
    PAUSE: 'pauseNotification',
    RESUME: 'resumeNotification',
    CLEAR: 'clearNotifications',
};

export const PM_ACTIONS = {
    ADD: 'addProjectMembers',
    DELETE: 'deleteProjectMembers',
    TOGGLE_SELECTION: 'toggleProjectMemberSelection',
    CLEAR_SELECTION: 'clearProjectMemberSelection',
    FETCH: 'fetchProjectMembers',
    CLEAR: 'clearProjectMembers',
    SET_SEARCH_QUERY: 'setProjectMembersSearchQuery',
    SET_SORT_BY: 'setProjectMembersSortingBy',
    SET_SORT_DIRECTION: 'setProjectMembersSortingDirection',
    CLEAR_OFFSET: 'clearProjectMembersOffset',
};
