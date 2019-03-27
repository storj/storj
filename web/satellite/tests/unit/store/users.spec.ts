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
                fullName: '',
                shortName: '',
                email: '',
            }
        };

        const store = new Vuex.Store({state, mutations});

        const user = {
            fullName: 'fullName',
            shortName: 'shortName',
            email: 'email',
        };

        store.commit(USER_MUTATIONS.SET_USER_INFO, user);

        expect(state.user.email).toBe('email');
        expect(state.user.fullName).toBe('fullName');
        expect(state.user.shortName).toBe('shortName');
    });

    it('clear user info', () => {
        const state = {
            user: {
                fullName: 'fullName',
                shortName: 'shortName',
                email: 'email',
            }
        };

        const store = new Vuex.Store({state, mutations});

        store.commit(USER_MUTATIONS.REVERT_TO_DEFAULT_USER_INFO);

        expect(state.user.email).toBe('');
        expect(state.user.fullName).toBe('');
        expect(state.user.shortName).toBe('');
    });

    it('Update user info', () => {
        const state = {
            user: {
                fullName: '',
                shortName: '',
                email: '',
            }
        };
        const user = {
            fullName: 'fullName',
            shortName: 'shortName',
            email: 'email',
        };

        const store = new Vuex.Store({state, mutations});

        store.commit(USER_MUTATIONS.UPDATE_USER_INFO, user);

        expect(state.user.email).toBe('email');
        expect(state.user.fullName).toBe('fullName');
        expect(state.user.shortName).toBe('shortName');
    });
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
    });
    it('success update account', async () => {
        jest.spyOn(api, 'updateAccountRequest').mockReturnValue(
            Promise.resolve(<RequestResponse<User>>{
                isSuccess: true, data: {
                    fullName: 'fullName',
                    shortName: 'shortName',
                    email: 'email',
                }
            })
        );
        const commit = jest.fn();
        const user = {
            fullName: '',
            shortName: '',
            email: '',
        };

        const dispatchResponse = await usersModule.actions.updateAccount({commit}, user);

        expect(dispatchResponse.isSuccess).toBeTruthy();
        expect(commit).toHaveBeenCalledWith(USER_MUTATIONS.UPDATE_USER_INFO, {
            fullName: 'fullName',
            shortName: 'shortName',
            email: 'email',
        });
    });

    it('error update account', async () => {
        jest.spyOn(api, 'updateAccountRequest').mockReturnValue(
            Promise.resolve(<RequestResponse<User>>{
                isSuccess: false
            })
        );
        const commit = jest.fn();
        const user = {
            fullName: '',
            shortName: '',
            email: '',
        };

        const dispatchResponse = await usersModule.actions.updateAccount({commit}, user);

        expect(dispatchResponse.isSuccess).toBeFalsy();
        expect(commit).toHaveBeenCalledTimes(0);
    });

    it('password change', async () => {
        jest.spyOn(api, 'changePasswordRequest').mockReturnValue(
            Promise.resolve(<RequestResponse<null>>{
                isSuccess: true
            })
        );
        const commit = jest.fn();
        const updatePasswordModel = {oldPassword: 'o', newPassword: 'n'};

        const requestResponse = await usersModule.actions.changePassword({commit}, updatePasswordModel);

        expect(requestResponse.isSuccess).toBeTruthy();
    });

    it('delete account', async () => {
        jest.spyOn(api, 'deleteAccountRequest').mockReturnValue(
            Promise.resolve(<RequestResponse<null>>{
                isSuccess: true
            })
        );

        const commit = jest.fn();
        const password = '';

        const dispatchResponse = await usersModule.actions.deleteAccount(commit, password);

        expect(dispatchResponse.isSuccess).toBeTruthy();
    });

    it('success get user', async () => {
        jest.spyOn(api, 'getUserRequest').mockReturnValue(
            Promise.resolve(<RequestResponse<User>>{
                isSuccess: true,
                data: {
                    fullName: '',
                    shortName: '',
                    email: '',
                }
            })
        );
        const commit = jest.fn();

        const requestResponse = await usersModule.actions.getUser({commit});

        expect(requestResponse.isSuccess).toBeTruthy();
    });

    it('error get user', async () => {
        jest.spyOn(api, 'getUserRequest').mockReturnValue(
            Promise.resolve(<RequestResponse<User>>{
                isSuccess: false
            })
        );
        const commit = jest.fn();

        const requestResponse = await usersModule.actions.getUser({commit});

        expect(requestResponse.isSuccess).toBeFalsy();
    });
});

describe('getters', () => {
    it('user model', function () {
        const state = {
            user: {
                fullName: 'fullName',
                shortName: 'shortName',
                email: 'email',
            }
        };

        const retrievedUser = usersModule.getters.user(state);

        expect(retrievedUser.fullName).toBe('fullName');
        expect(retrievedUser.shortName).toBe('shortName');
        expect(retrievedUser.email).toBe('email');
    });

    it('user name', function () {
        const state = {
            user: {
                fullName: 'John',
                shortName: 'Doe'
            }
        };

        const retrievedUserName = usersModule.getters.userName(state);

        expect(retrievedUserName).toBe('John Doe');
    });
});
