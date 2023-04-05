// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed, reactive } from 'vue';

import {
    DisableMFARequest,
    SetUserSettingsData,
    UpdatedUser,
    User,
    UsersApi,
    UserSettings,
} from '@/types/users';
import { AuthHttpApi } from '@/api/auth';
import { useAppStore } from '@/store/modules/appStore';

export class UsersState {
    public user: User = new User();
    public settings: UserSettings = new UserSettings();
    public userMFASecret = '';
    public userMFARecoveryCodes: string[] = [];
}

export const useUsersStore = defineStore('users', () => {
    const state = reactive<UsersState>(new UsersState());

    const appStore = useAppStore();

    const userName = computed(() => {
        return state.user.getFullName();
    });

    const shouldOnboard = computed(() => {
        return !state.settings.onboardingStart || (state.settings.onboardingStart && !state.settings.onboardingEnd);
    });

    const api: UsersApi = new AuthHttpApi();

    async function updateUser(userInfo: UpdatedUser): Promise<void> {
        await api.update(userInfo);

        state.user.fullName = userInfo.fullName;
        state.user.shortName = userInfo.shortName;
    }

    async function getUser(): Promise<void> {
        const user = await api.get();
        user.projectLimit ||= appStore.state.config.defaultProjectLimit;

        setUser(user);
    }

    async function getFrozenStatus(): Promise<void> {
        state.user.isFrozen = await api.getFrozenStatus();
    }

    async function disableUserMFA(request: DisableMFARequest): Promise<void> {
        await api.disableUserMFA(request.passcode, request.recoveryCode);
    }

    async function enableUserMFA(passcode: string): Promise<void> {
        await api.enableUserMFA(passcode);
    }

    async function generateUserMFASecret(): Promise<void> {
        state.userMFASecret = await api.generateUserMFASecret();
    }

    async function generateUserMFARecoveryCodes(): Promise<void> {
        const codes = await api.generateUserMFARecoveryCodes();

        state.userMFARecoveryCodes = codes;
        state.user.mfaRecoveryCodeCount = codes.length;
    }

    async function getSettings(): Promise<UserSettings> {
        const settings = await api.getUserSettings();

        state.settings = settings;

        return settings;
    }

    async function updateSettings(update: SetUserSettingsData): Promise<void> {
        state.settings = await api.updateSettings(update);
    }

    function setUser(user: User): void {
        state.user = user;
    }

    // Does nothing. It is called on login screen, and we just subscribe to this action in dashboard wrappers.
    function login(): void {}

    function clear() {
        state.user = new User();
        state.settings = new UserSettings();
        state.userMFASecret = '';
        state.userMFARecoveryCodes = [];
    }

    return {
        state,
        userName,
        shouldOnboard,
        updateUser,
        getUser,
        disableUserMFA,
        enableUserMFA,
        generateUserMFASecret,
        generateUserMFARecoveryCodes,
        clear,
        login,
        getFrozenStatus,
        setUser,
        updateSettings,
        getSettings,
    };
});
