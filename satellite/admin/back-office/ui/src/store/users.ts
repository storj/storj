// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { reactive } from 'vue';

import {
    AccountMin,
    ChangeHistoryHttpApiV1,
    ChangeLog,
    CreateRestKeyRequest,
    DisableUserRequest,
    FreezeEventType,
    KindInfo,
    ToggleFreezeUserRequest,
    ToggleMfaRequest,
    UpdateUserRequest,
    UserAccount,
    UserManagementHttpApiV1,
    UserStatusInfo,
} from '@/api/client.gen';
import { ItemType } from '@/types/common';

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
    const changeHistoryApi = new ChangeHistoryHttpApiV1();

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
        if (state.freezeTypes.length) {
            return;
        }
        state.freezeTypes = await userApi.getFreezeEventTypes();
    }

    async function freezeUser(userID: string, eventType: number, reason: string): Promise<void> {
        const request = new ToggleFreezeUserRequest();
        request.action = 'freeze';
        request.type = eventType;
        request.reason = reason;
        await userApi.toggleFreezeUser(request, userID);
    }

    async function unfreezeUser(userID: string, reason: string): Promise<void> {
        const request = new ToggleFreezeUserRequest();
        request.action = 'unfreeze';
        request.reason = reason;
        await userApi.toggleFreezeUser(request, userID);
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

    async function deleteUser(userID: string, markPendingDeletion: boolean, reason: string): Promise<UserAccount> {
        const request = new DisableUserRequest();
        request.reason = reason;
        request.setPendingDeletion = markPendingDeletion;
        return await userApi.disableUser(request, userID);
    }

    async function disableMFA(userID: string, reason: string): Promise<void> {
        const request = new ToggleMfaRequest();
        request.reason = reason;
        await userApi.toggleMFA(request, userID);
    }

    async function createRestKey(userID: string, expirationDate: Date, reason: string): Promise<string> {
        const request = new CreateRestKeyRequest();
        request.reason = reason;
        request.expiration = expirationDate.toISOString();
        return await userApi.createRestKey(request, userID);
    }

    async function findUsers(param: string): Promise<void> {
        state.searchResults =  await userApi.searchUsers(param);
    }

    function setSearchTerm(term: string): void {
        state.searchTerm = term;
        state.searchResults = [];
    }

    async function getHistory(userID: string, exact = true): Promise<ChangeLog[]> {
        return await changeHistoryApi.getChangeHistory(`${exact}`, ItemType.User, userID);
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
        createRestKey,
        getHistory,
    };
});
