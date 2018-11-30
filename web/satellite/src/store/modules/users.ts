// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    USER_MUTATIONS,
} from "../mutationConstants";
import {updateCompanyInfo, updatePassword, updateBasicUserInfo, deleteUserAccount} from "@/api/users";

export const authModule = {
    state: {
        user: {
            firstName: "",
            lastName: "",
            email: "",
            id: "",
            company: {
                name: "",
                address: "",
                country: "",
                city: "",
                state: "",
                postalCode: "",
            }
        }
    },

    mutations: {
        [USER_MUTATIONS.SET_USER_INFO](state: any, user: User): void {
            state.user.firstName = user.firstName;
            state.user.lastName = user.lastName;
            state.user.email = user.email;
            state.user.id = user.id;
            state.user.company.name = user.company.name;
            state.user.company.address = user.company.address;
            state.user.company.country = user.company.country;
            state.user.company.city = user.company.city;
            state.user.company.state = user.company.state;
            state.user.company.postalCode = user.company.postalCode;
        },

        [USER_MUTATIONS.REVERT_TO_DEFAULT_USER_INFO](state: any): void {
            state.user.firstName = "";
            state.user.lastName = "";
            state.user.email = "";
            state.user.id = "";
            state.user.company.name = "";
            state.user.company.address = "";
            state.user.company.country = "";
            state.user.company.city = "";
            state.user.company.state = "";
            state.user.company.postalCode = "";
        },

        [USER_MUTATIONS.UPDATE_USER_INFO](state: any, user: User): void {
            state.user.firstName = user.firstName;
            state.user.lastName = user.lastName;
            state.user.email = user.email;
        },

        [USER_MUTATIONS.UPDATE_COMPANY_INFO](state: any, company: Company): void {
            state.user.company.name = company.name;
            state.user.company.address = company.address;
            state.user.company.country = company.country;
            state.user.company.city = company.city;
            state.user.company.state = company.state;
            state.user.company.postalCode = company.postalCode;
        },
    },

    actions: {
        setUserInfo: setUserInfo,
        updateBasicUserInfo: async function ({commit}: any, userInfo: User): Promise<boolean>{
            let response = await updateBasicUserInfo(userInfo);

            if (!response || !response.data) {
                return false;
            }

            commit(USER_MUTATIONS.UPDATE_USER_INFO, userInfo)

            return true;
        },
        updateCompanyInfo: async function ({commit}: any, userInfo: User): Promise<boolean>{
            let response = await updateCompanyInfo(userInfo.id, userInfo.company);

            if (!response || !response.data) {
                return false;
            }

            commit(USER_MUTATIONS.UPDATE_COMPANY_INFO, response.data.updateCompany)

            return true;
        },
        updatePassword: async function ({state}: any, password: string): Promise<boolean> {
            let response = await updatePassword(state.user.id, password);

            if (!response) {
                console.log("error during password change");
                return false;
            }

            return true;
        },
        deleteUserAccount: async function ({commit, state}: any):Promise<boolean> {
            let response = await deleteUserAccount(state.user.id);

            if (!response) {
                console.log("error during account delete");
                return false;
            }

            return true;
        }
    },

    getters: {
        user: (state: any) => {
            return state.user;
        },
        userName: (state: any) => `${state.user.firstName} ${state.user.lastName}`
    },
};

function setUserInfo({commit}: any, userInfo: User): void {
    commit(USER_MUTATIONS.SET_USER_INFO, userInfo)
}
