// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

import { USER_MUTATIONS, } from '../mutationConstants';
import {
	deleteUserAccountRequest,
	updateBasicUserInfoRequest,
	updatePasswordRequest,
    getUserRequest
} from '@/api/users';

export const usersModule = {
	state: {
		user: {
			firstName: '',
			lastName: '',
			email: '',
			id: '',
		}
	},

	mutations: {
		[USER_MUTATIONS.SET_USER_INFO](state: any, user: User): void {
			state.user.firstName = user.firstName;
			state.user.lastName = user.lastName;
			state.user.email = user.email;
			state.user.id = user.id;
		},

		[USER_MUTATIONS.REVERT_TO_DEFAULT_USER_INFO](state: any): void {
			state.user.firstName = '';
			state.user.lastName = '';
			state.user.email = '';
			state.user.id = '';
		},

		[USER_MUTATIONS.UPDATE_USER_INFO](state: any, user: User): void {
			state.user.firstName = user.firstName;
			state.user.lastName = user.lastName;
			state.user.email = user.email;
		},
	},

	actions: {
		updateBasicUserInfo: async function ({commit}: any, userInfo: User): Promise<boolean> {
			let response = await updateBasicUserInfoRequest(userInfo);

			if (!response || !response.data) {
				return false;
			}

			commit(USER_MUTATIONS.UPDATE_USER_INFO, userInfo);

			return true;
		},
		updatePassword: async function ({state}: any, password: string): Promise<boolean> {
			let response = await updatePasswordRequest(state.user.id, password);

			return response !== null;
		},
		deleteUserAccount: async function ({commit, state}: any, password: string): Promise<boolean> {
			let response = await deleteUserAccountRequest(password);

            return response !== null;
		},
		getUser: async function ({commit}: any): Promise<boolean> {
			let response = await getUserRequest();

			if (!response) {
				return false;
			}

            commit(USER_MUTATIONS.SET_USER_INFO, response.data.user);

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
