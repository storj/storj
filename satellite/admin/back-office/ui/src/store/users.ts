// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

import {
    AccountMin,
    FreezeEventType,
    FreezeUserRequest,
    KindInfo,
    UpdateUserRequest,
    UserAccount,
    UserManagementHttpApiV1,
    UserStatusInfo,
} from '@/api/client.gen';

class UsersState {
    public currentAccount: UserAccount | null = null;
    public freezeTypes: FreezeEventType[] = [];
    public userKinds: KindInfo[] = [];
    public userStatuses: UserStatusInfo[] = [];
    public searchResults: AccountMin[] = [];
    public searchTerm = '';
}

export const useUsersStore = defineStore('users', () => {
    const state = reactive<UsersState>(new UsersState());

    const userApi = new UserManagementHttpApiV1();

    async function getUserByEmail(email: string): Promise<UserAccount> {
        return await userApi.getUserByEmail(email);
    }

    async function getUser(userID: string): Promise<UserAccount> {
        return await userApi.getUser(userID);
    }

    // Update the current user stored in the state.
    async function updateCurrentUser(user: string | UserAccount): Promise<void> {
        if (typeof user === 'string') {
            state.currentAccount = await getUser(user);
            return;
        }
        state.currentAccount = user;
    }

    function clearCurrentUser(): void {
        state.currentAccount = null;
    }

    async function getAccountFreezeTypes(): Promise<void> {
        state.freezeTypes = await userApi.getFreezeEventTypes();
    }

    async function freezeUser(userID: string, eventType: number): Promise<void> {
        const request = new FreezeUserRequest();
        request.type = eventType;
        await userApi.freezeUser(request, userID);
    }

    async function unfreezeUser(userID: string): Promise<void> {
        await userApi.unfreezeUser(userID);
    }

    async function getUserKinds(): Promise<void> {
        if (state.userKinds.length) {
            return;
        }
        state.userKinds = await userApi.getUserKinds();
    }

    async function getUserStatuses(): Promise<void> {
        if (state.userStatuses.length) {
            return;
        }
        state.userStatuses = await userApi.getUserStatuses();
    }

    // Update a specified user.
    async function updateUser(userID: string, request: UpdateUserRequest): Promise<UserAccount> {
        return await userApi.updateUser(request, userID);
    }

    async function deleteUser(userID: string): Promise<void> {
        await userApi.deleteUser(userID);
    }

    async function disableMFA(userID: string): Promise<void> {
        await userApi.disableMFA(userID);
    }

    async function findUsers(param: string): Promise<void> {
        state.searchResults =  await userApi.searchUsers(param);
    }

    function setSearchTerm(term: string): void {
        state.searchTerm = term;
        state.searchResults = [];
    }

    return {
        state,
        setSearchTerm,
        findUsers,
        getUserByEmail,
        updateCurrentUser,
        clearCurrentUser,
        getAccountFreezeTypes,
        freezeUser,
        unfreezeUser,
        getUserKinds,
        getUserStatuses,
        updateUser,
        deleteUser,
        disableMFA,
    };
});
