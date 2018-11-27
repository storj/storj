// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    SET_USER_INFO,
    REVERT_TO_DEFAULT_USER_INFO,
} from "../mutationConstants";

export const authModule = {
    state: {
        firstName: "",
        lastName: "",
        email: "",
        id: "",
        companyName: "",
        companyAddress: "",
        companyCountry: "",
        companyCity: "",
        companyState: "",
        companyPostalCode: "",
    },

    mutations: {
        [SET_USER_INFO](state: any, user: User): void {
            state.firstName = user.firstName;
            state.lastName = user.lastName;
            state.email = user.email;
            state.id = user.id;
            state.companyName = user.company.name;
            state.companyAddress = user.company.address;
            state.companyCountry = user.company.country;
            state.companyCity = user.company.city;
            state.companyState = user.company.state;
            state.companyPostalCode = user.company.postalCode;
        },

        [REVERT_TO_DEFAULT_USER_INFO](state: any): void {
            state.firstName = "";
            state.lastName = "";
            state.email = "";
            state.id = "";
            state.companyName = "";
            state.companyAddress = "";
            state.companyCountry = "";
            state.companyCity = "";
            state.companyState = "";
            state.companyPostalCode = "";
        },
    },

    actions: {
        setUserInfo: setUserInfo,
    },

    getters: {
        userName: (state: any) => `${state.firstName} ${state.lastName}`
    },
};

function setUserInfo({commit}: any, userInfo: User): void {
    commit(SET_USER_INFO, userInfo)
}
