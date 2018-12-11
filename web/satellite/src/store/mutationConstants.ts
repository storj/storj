// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

export const USER_MUTATIONS = {
    SET_USER_INFO: "SET_USER_INFO",
    REVERT_TO_DEFAULT_USER_INFO: "REVERT_TO_DEFAULT_USER_INFO",
    UPDATE_USER_INFO: "UPDATE_USER_INFO",
    UPDATE_COMPANY_INFO: "UPDATE_COMPANY_INFO",
};

export const PROJECTS_MUTATIONS = {
    CREATE: "CREATE_PROJECT",
    DELETE: "DELETE_PROJECT",
    UPDATE: "UPDATE_PROJECT",
    FETCH:  "FETCH_PROJECTS",
    SELECT: "SELECT_PROJECT",
};

export const NOTIFICATION_MUTATIONS = {
    ADD: 'ADD_NOTIFICATION',
    DELETE: 'DELETE_NOTIFICATION',
    PAUSE: 'PAUSE_NOTIFICATION',
    RESUME: 'RESUME_NOTIFICATION',
};
