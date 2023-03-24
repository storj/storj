// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export const APP_STATE_ACTIONS = {
    TOGGLE_SUCCESSFUL_PASSWORD_RESET: 'TOGGLE_SUCCESSFUL_PASSWORD_RESET',
    CLOSE_POPUPS: 'closePopups',
    CLEAR: 'clearAppstate',
    CHANGE_FETCH_STATE: 'changeFetchState',
    SET_SATELLITE_NAME: 'SET_SATELLITE_NAME',
    SET_PARTNERED_SATELLITES: 'SET_PARTNERED_SATELLITES',
    SET_SATELLITE_STATUS: 'SET_SATELLITE_STATUS',
    SET_COUPON_CODE_BILLING_UI_STATUS: 'SET_COUPON_CODE_BILLING_UI_STATUS',
    SET_COUPON_CODE_SIGNUP_UI_STATUS: 'SET_COUPON_CODE_SIGNUP_UI_STATUS',
    SET_ENCRYPTION_PASSPHRASE_FLOW_STATUS: 'SET_ENCRYPTION_PASSPHRASE_FLOW_STATUS',
    TOGGLE_ACTIVE_DROPDOWN: 'TOGGLE_ACTIVE_DROPDOWN',
    FETCH_CONFIG: 'FETCH_CONFIG',
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
