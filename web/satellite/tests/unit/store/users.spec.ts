// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { usersModule } from '@/store/modules/users';
import * as api from '@/api/users';
import { changePasswordRequest, deleteAccountRequest, getUserRequest, updateAccountRequest } from '@/api/users';
import { USER_MUTATIONS } from '@/store/mutationConstants';
import Vuex from 'vuex';
import { createLocalVue } from '@vue/test-utils';

const mutations = usersModule.mutations;

describe('mutations', () => {
	beforeEach(() => {
		createLocalVue().use(Vuex);
	});
	it('Set user info', () => {
		const state = {
			user: {
				firstName: '',
				lastName: '',
				email: ''
			}
		};

		const store = new Vuex.Store({state, mutations});

		const user = {
			firstName: 'firstName',
			lastName: 'lastName',
			email: 'email'
		};

		store.commit(USER_MUTATIONS.SET_USER_INFO, user);

		expect(state.user.email).toBe('email');
		expect(state.user.firstName).toBe('firstName');
		expect(state.user.lastName).toBe('lastName');
	});

	it('clear user info', () => {
		const state = {
			user: {
				firstName: 'firstName',
				lastName: 'lastName',
				email: 'email',
			}
		};

		const store = new Vuex.Store({state, mutations});

		store.commit(USER_MUTATIONS.REVERT_TO_DEFAULT_USER_INFO);

		expect(state.user.email).toBe('');
		expect(state.user.firstName).toBe('');
		expect(state.user.lastName).toBe('');
	});

	it('Update user info', () => {
		const state = {
			user: {
				firstName: '',
				lastName: '',
				email: ''
			}
		};
		const user = {
			firstName: 'firstName',
			lastName: 'lastName',
			email: 'email'
		};

		const store = new Vuex.Store({state, mutations});

		store.commit(USER_MUTATIONS.UPDATE_USER_INFO, user);

		expect(state.user.email).toBe('email');
		expect(state.user.firstName).toBe('firstName');
		expect(state.user.lastName).toBe('lastName');
	});
});

describe('actions', () => {
	beforeEach(() => {
		jest.resetAllMocks();
	});
	it('success update account', async () => {
		jest.spyOn(api, 'updateAccountRequest').mockReturnValue(
			{
				isSuccess: true, data: {
					firstName: 'firstName',
					lastName: 'lastName',
					email: 'email',
				}
			});
		const commit = jest.fn();
		const user = {
			firstName: '',
			lastName: '',
			email: ''
		};

		const dispatchResponse = await usersModule.actions.updateAccount({commit}, user);

		expect(dispatchResponse.isSuccess).toBeTruthy();
		expect(commit).toHaveBeenCalledWith(USER_MUTATIONS.UPDATE_USER_INFO, {
			firstName: 'firstName',
			lastName: 'lastName',
			email: 'email'
		});
	});

	it('error update account', async () => {
		jest.spyOn(api, 'updateAccountRequest').mockReturnValue(
			{
				isSuccess: false
			});
		const commit = jest.fn();
		const user = {
			firstName: '',
			lastName: '',
			email: ''
		};

		const dispatchResponse = await usersModule.actions.updateAccount({commit}, user);

		expect(dispatchResponse.isSuccess).toBeFalsy();
		expect(commit).toHaveBeenCalledTimes(0);
	});

	it('password change', async () => {
		jest.spyOn(api, 'changePasswordRequest').mockReturnValue({isSuccess: true});
		const commit = jest.fn();
		const updatePasswordModel = {oldPassword: 'o', newPassword: 'n'};

		const requestResponse = await usersModule.actions.changePassword({commit}, updatePasswordModel);

		expect(requestResponse.isSuccess).toBeTruthy();
	});

	it('delete account', async () => {
		jest.spyOn(api, 'deleteAccountRequest').mockReturnValue({isSuccess: true});
		const commit = jest.fn();
		const password = '';

		const dispatchResponse = await usersModule.actions.deleteAccount(commit, password);

		expect(dispatchResponse.isSuccess).toBeTruthy();
	});

	it('success get user', async () => {
		jest.spyOn(api, 'getUserRequest').mockReturnValue({
			isSuccess: true,
			data: {
				firstName: '',
				lastName: '',
				email: ''
			}
		});
		const commit = jest.fn();

		const requestResponse = await usersModule.actions.getUser({commit});

		expect(requestResponse.isSuccess).toBeTruthy();
	});

	it('error get user', async () => {
		jest.spyOn(api, 'getUserRequest').mockReturnValue({isSuccess: false});
		const commit = jest.fn();

		const requestResponse = await usersModule.actions.getUser({commit});

		expect(requestResponse.isSuccess).toBeFalsy();
	});
});

describe('getters', () => {
	it('user model', function () {
		const state = {
			user: {
				firstName: 'firstName',
				lastName: 'lastName',
				email: 'email',
			}
		};

		const retrievedUser = usersModule.getters.user(state);

		expect(retrievedUser.firstName).toBe('firstName');
		expect(retrievedUser.lastName).toBe('lastName');
		expect(retrievedUser.email).toBe('email');
	});

	it('user name', function () {
		const state = {
			user: {
				firstName: 'John',
				lastName: 'Doe'
			}
		};

		const retrievedUserName = usersModule.getters.userName(state);

		expect(retrievedUserName).toBe('John Doe');
	});
});
