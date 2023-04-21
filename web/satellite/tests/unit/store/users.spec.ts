// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

import { createPinia, setActivePinia } from 'pinia';

import { AuthHttpApi } from '@/api/auth';
import { UpdatedUser, User } from '@/types/users';
import { useUsersStore } from '@/store/modules/usersStore';

describe('actions', () => {
    beforeEach(() => {
        setActivePinia(createPinia());
        jest.resetAllMocks();
    });

    it('set user', () => {
        const store = useUsersStore();
        const user = new User('1', 'fullName', 'shortName', 'user@example.com');

        store.setUser(user);

        expect(store.state.user.id).toBe(user.id);
        expect(store.state.user.email).toBe(user.email);
        expect(store.state.user.fullName).toBe(user.fullName);
        expect(store.state.user.shortName).toBe(user.shortName);
    });

    it('clear user', () => {
        const store = useUsersStore();

        const user = new User('1', 'fullName', 'shortName', 'user@example.com');

        store.setUser(user);
        expect(store.state.user.id).toBe(user.id);

        store.clear();
        expect(store.state.user.id).toBe('');
        expect(store.state.user.email).toBe('');
        expect(store.state.user.fullName).toBe('');
        expect(store.state.user.shortName).toBe('');
    });

    it('update user', async () => {
        const store = useUsersStore();
        const user = new UpdatedUser('fullName', 'shortName');

        jest.spyOn(AuthHttpApi.prototype, 'update').mockImplementation(() => Promise.resolve());

        await store.updateUser(user);

        expect(store.state.user.fullName).toBe(user.fullName);
        expect(store.state.user.shortName).toBe(user.shortName);
    });

    it('update throws an error when api call fails', async () => {
        const store = useUsersStore();
        const newUser = new UpdatedUser('', '');
        const oldUser = store.state.user;

        jest.spyOn(AuthHttpApi.prototype, 'update').mockImplementation(() => { throw new Error(); });

        try {
            await store.updateUser(newUser);
            expect(true).toBe(false);
        } catch (error) {
            expect(store.state.user.fullName).toBe(oldUser.fullName);
            expect(store.state.user.shortName).toBe(oldUser.shortName);
        }
    });

    it('success get user', async () => {
        const store = useUsersStore();
        const user = new User('2', 'newFullName', 'newShortName', 'user2@example.com');

        jest.spyOn(AuthHttpApi.prototype, 'get').mockReturnValue(
            Promise.resolve(user),
        );

        await store.getUser();

        expect(store.state.user.id).toBe(user.id);
        expect(store.state.user.shortName).toBe(user.shortName);
        expect(store.state.user.fullName).toBe(user.fullName);
        expect(store.state.user.email).toBe(user.email);
    });

    it('get throws an error when api call fails', async () => {
        const store = useUsersStore();
        const user = store.state.user;

        jest.spyOn(AuthHttpApi.prototype, 'get').mockImplementation(() => { throw new Error(); });

        try {
            await store.getUser();
            expect(true).toBe(false);
        } catch (error) {
            expect(store.state.user.fullName).toBe(user.fullName);
            expect(store.state.user.shortName).toBe(user.shortName);
        }
    });
});

describe('getters', () => {
    beforeEach(() => {
        setActivePinia(createPinia());
    });

    it('user name', function () {
        const store = useUsersStore();
        const retrievedUserName = store.userName;

        expect(retrievedUserName).toBe(store.state.user.getFullName());
    });
});
