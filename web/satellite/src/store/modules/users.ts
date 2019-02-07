// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { USER_MUTATIONS } from '../mutationConstants';
import {
	deleteAccountRequest,
	updateAccountRequest,
	changePasswordRequest,
	getUserRequest,
	activateAccountRequest
} from '@/api/users';

export const usersModule = {
	state: {
		user: {
			firstName: '',
			lastName: '',
			email: ''
		}
	},

	mutations: {
		[USER_MUTATIONS.SET_USER_INFO](state: any, user: User): void {
			state.user = user;
		},

		[USER_MUTATIONS.REVERT_TO_DEFAULT_USER_INFO](state: any): void {
			state.user.firstName = '';
			state.user.lastName = '';
			state.user.email = '';
		},

        [USER_MUTATIONS.UPDATE_USER_INFO](state: any, user: User): void {
            state.user = user;
        },

        [USER_MUTATIONS.CLEAR](state: any): void {
            state.user = {
                firstName: '',
                lastName: '',
                email: ''
            };
        },
	},

	actions: {
        updateAccount: async function ({commit}: any, userInfo: User): Promise<RequestResponse<User>> {
			let response = await updateAccountRequest(userInfo);
            
			if (response.isSuccess) {
                commit(USER_MUTATIONS.UPDATE_USER_INFO, response.data);
			}

			return response;
		},
        changePassword: async function ({state}: any, updateModel: UpdatePasswordModel): Promise<RequestResponse<null>> {
			return await changePasswordRequest(updateModel.oldPassword, updateModel.newPassword);
		},
		deleteAccount: async function ({commit, state}: any, password: string): Promise<RequestResponse<null>> {
            return await deleteAccountRequest(password);
		},
		getUser: async function ({commit}: any): Promise<RequestResponse<User>> {
			let response = await getUserRequest();

			if (response.isSuccess) {
                commit(USER_MUTATIONS.SET_USER_INFO, response.data);
			}

			return response;
		},
        clearUser: function({commit}: any) {
            commit(USER_MUTATIONS.CLEAR);
        },
		activateAccount: async function ({commit}, temporaryToken: string): Promise<RequestResponse<string>> {
			return await activateAccountRequest(temporaryToken);
		}
	},

	getters: {
		user: (state: any) => {
			return state.user;
		},
		userName: (state: any) => `${state.user.firstName} ${state.user.lastName}`
	},
};
