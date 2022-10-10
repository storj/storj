// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export const APP_STATE_ACTIONS = {
    TOGGLE_ACCOUNT: 'toggleAccountDropdown',
    TOGGLE_SELECT_PROJECT_DROPDOWN: 'toggleSelectProjectDropdown',
    TOGGLE_RESOURCES_DROPDOWN: 'toggleResourcesDropdown',
    TOGGLE_QUICK_START_DROPDOWN: 'toggleQuickStartDropdown',
    TOGGLE_SETTINGS_DROPDOWN: 'toggleSettingsDropdown',
    TOGGLE_EDIT_PROJECT_DROPDOWN: 'toggleEditProjectDropdown',
    TOGGLE_FREE_CREDITS_DROPDOWN: 'toggleFreeCreditsDropdown',
    TOGGLE_AVAILABLE_BALANCE_DROPDOWN: 'toggleAvailableBalanceDropdown',
    TOGGLE_PERIODS_DROPDOWN: 'togglePeriodsDropdown',
    TOGGLE_AG_DATEPICKER_DROPDOWN: 'toggleAGDatepickerDropdown',
    TOGGLE_CHARTS_DATEPICKER_DROPDOWN: 'toggleChartsDatepickerDropdown',
    TOGGLE_BUCKET_NAMES_DROPDOWN: 'toggleBucketNamesDropdown',
    TOGGLE_PERMISSIONS_DROPDOWN: 'togglePermissionsDropdown',
    TOGGLE_SUCCESSFUL_PASSWORD_RESET: 'TOGGLE_SUCCESSFUL_PASSWORD_RESET',
    TOGGLE_SUCCESSFUL_PROJECT_CREATION_POPUP: 'toggleSuccessfulProjectCreationPopup',
    TOGGLE_UPLOAD_CANCEL_POPUP: 'toggleUploadCancelPopup',
    SHOW_SET_DEFAULT_PAYMENT_METHOD_POPUP: 'showSetDefaultPaymentMethodPopup',
    CLOSE_SET_DEFAULT_PAYMENT_METHOD_POPUP: 'closeSetDefaultPaymentMethodPopup',
    SHOW_DELETE_PAYMENT_METHOD_POPUP: 'showDeletePaymentMethodPopup',
    CLOSE_DELETE_PAYMENT_METHOD_POPUP: 'closeDeletePaymentMethodPopup',
    CLOSE_POPUPS: 'closePopups',
    CHANGE_STATE: 'changeFetchState',
    TOGGLE_PAYMENT_SELECTION: 'TOGGLE_PAYMENT_SELECTION',
    SET_SATELLITE_NAME: 'SET_SATELLITE_NAME',
    SET_PARTNERED_SATELLITES: 'SET_PARTNERED_SATELLITES',
    SET_SATELLITE_STATUS: 'SET_SATELLITE_STATUS',
    SET_COUPON_CODE_BILLING_UI_STATUS: 'SET_COUPON_CODE_BILLING_UI_STATUS',
    SET_COUPON_CODE_SIGNUP_UI_STATUS: 'SET_COUPON_CODE_SIGNUP_UI_STATUS',
    SET_PROJECT_DASHBOARD_STATUS: 'SET_PROJECT_DASHBOARD_STATUS',
    SET_OBJECTS_FLOW_STATUS: 'SET_OBJECTS_FLOW_STATUS',
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
