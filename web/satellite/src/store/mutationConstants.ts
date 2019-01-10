// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

export const USER_MUTATIONS = {
	SET_USER_INFO: 'SET_USER_INFO',
	REVERT_TO_DEFAULT_USER_INFO: 'REVERT_TO_DEFAULT_USER_INFO',
	UPDATE_USER_INFO: 'UPDATE_USER_INFO',
    CLEAR: 'CLEAR_USER',
};

export const PROJECTS_MUTATIONS = {
	CREATE: 'CREATE_PROJECT',
	DELETE: 'DELETE_PROJECT',
	UPDATE: 'UPDATE_PROJECT',
	FETCH: 'FETCH_PROJECTS',
	SELECT: 'SELECT_PROJECT',
    CLEAR: 'CLEAR_PROJECTS',
};

export const PROJECT_MEMBER_MUTATIONS = {
	FETCH: 'FETCH_MEMBERS',
	TOGGLE_SELECTION: 'TOGGLE_SELECTION',
	CLEAR_SELECTION: 'CLEAR_SELECTION',
	ADD: 'ADD_MEMBERS',
	DELETE: 'DELETE_MEMBERS',
	CLEAR: 'CLEAR_MEMBERS',
	CHANGE_SORT_ORDER: 'CHANGE_SORT_ORDER',
	SET_SEARCH_QUERY: 'SET_SEARCH_QUERY',
	CLEAR_OFFSET: 'CLEAR_OFFSET',
	ADD_OFFSET:'ADD_OFFSET',
};

export const NOTIFICATION_MUTATIONS = {
	ADD: 'ADD_NOTIFICATION',
	DELETE: 'DELETE_NOTIFICATION',
	PAUSE: 'PAUSE_NOTIFICATION',
	RESUME: 'RESUME_NOTIFICATION',
};

export const APP_STATE_MUTATIONS = {
    TOGGLE_ADD_TEAMMEMBER_POPUP: 'TOGGLE_ADD_TEAMMEMBER_POPUP',
	TOGGLE_NEW_PROJECT_POPUP: 'TOGGLE_NEW_PROJECT_POPUP',
    TOGGLE_PROJECT_DROPDOWN: 'TOGGLE_PROJECT_DROPDOWN',
    TOGGLE_ACCOUNT_DROPDOWN: 'TOGGLE_ACCOUNT_DROPDOWN',
    TOGGLE_DELETE_PROJECT_DROPDOWN: 'TOGGLE_DELETE_PROJECT_DROPDOWN',
    TOGGLE_DELETE_ACCOUNT_DROPDOWN: 'TOGGLE_DELETE_ACCOUNT_DROPDOWN',
	TOGGLE_SORT_PROJECT_MEMBERS_BY_DROPDOWN: 'TOGGLE_SORT_PROJECT_MEMBERS_BY_DROPDOWN',
    CLOSE_ALL: 'CLOSE_ALL',
};
