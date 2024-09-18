// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

import { defineStore } from 'pinia';
import { computed, DeepReadonly, reactive, readonly } from 'vue';

import {
    ACCOUNT_SETUP_STEPS,
    AccountDeletionData,
    DisableMFARequest,
    OnboardingStep,
    SessionsCursor,
    SessionsOrderBy,
    SessionsPage,
    SetUserSettingsData,
    UpdatedUser,
    User,
    UsersApi,
    UserSettings,
} from '@/types/users';
import { AuthHttpApi } from '@/api/auth';
import { useConfigStore } from '@/store/modules/configStore';
import { ChangeEmailStep, DeleteAccountStep } from '@/types/accountActions';
import { SortDirection } from '@/types/common';
import { DEFAULT_PAGE_LIMIT } from '@/types/pagination';

export const DEFAULT_USER_SETTINGS = readonly(new UserSettings());

export class UsersState {
    public user: User = new User();
    public settings: DeepReadonly<UserSettings> = DEFAULT_USER_SETTINGS;
    public userMFASecret = '';
    public userMFARecoveryCodes: string[] = [];
    public sessionsCursor: SessionsCursor = new SessionsCursor();
    public sessionsPage: SessionsPage = new SessionsPage();
}

export const useUsersStore = defineStore('users', () => {
    const state = reactive<UsersState>(new UsersState());

    const userName = computed(() => {
        return state.user.getFullName();
    });

    const shouldOnboard = computed(() => {
        return !state.settings.onboardingStart || (state.settings.onboardingStart && !state.settings.onboardingEnd);
    });

    const noticeDismissal = computed(() => state.settings.noticeDismissal);

    const api: UsersApi = new AuthHttpApi();

    async function updateUser(userInfo: UpdatedUser): Promise<void> {
        await api.update(userInfo);

        state.user.fullName = userInfo.fullName;
        state.user.shortName = userInfo.shortName;
    }

    async function changeEmail(step: ChangeEmailStep, data: string): Promise<void> {
        await api.changeEmail(step, data);
    }

    async function deleteAccount(step: DeleteAccountStep, data: string): Promise<AccountDeletionData | null> {
        return await api.deleteAccount(step, data);
    }

    async function getSessions(pageNumber: number, limit = DEFAULT_PAGE_LIMIT): Promise<SessionsPage> {
        state.sessionsCursor.page = pageNumber;
        state.sessionsCursor.limit = limit;

        const sessionsPage: SessionsPage = await api.getSessions(state.sessionsCursor);

        state.sessionsPage = sessionsPage;

        return sessionsPage;
    }

    async function invalidateSession(sessionID: string): Promise<void> {
        await api.invalidateSession(sessionID);
    }

    function setSessionsSortingBy(order: SessionsOrderBy): void {
        state.sessionsCursor.order = order;
    }

    function setSessionsSortingDirection(direction: SortDirection): void {
        state.sessionsCursor.orderDirection = direction;
    }

    async function getUser(): Promise<void> {
        const configStore = useConfigStore();

        const user = await api.get();
        user.freezeStatus = await api.getFrozenStatus();
        user.projectLimit ||= configStore.state.config.defaultProjectLimit;

        setUser(user);
    }

    function getShouldPromptPassphrase(isProjectOwner: boolean): boolean {
        const settings = state.settings;
        const step = settings.onboardingStep as OnboardingStep || OnboardingStep.AccountTypeSelection;
        if (!settings.passphrasePrompt) {
            return false;
        }
        if (!settings.onboardingEnd && ACCOUNT_SETUP_STEPS.includes(step)) {
            return false;
        }
        if (isProjectOwner) {
            return settings.onboardingEnd || step !== OnboardingStep.EncryptionPassphrase;
        }
        return true;
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

    async function regenerateUserMFARecoveryCodes(code: { recoveryCode?: string, passcode?: string }): Promise<void> {
        const codes = await api.regenerateUserMFARecoveryCodes(code.passcode, code.recoveryCode);

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

    async function requestProjectLimitIncrease(limit: string): Promise<void> {
        await api.requestProjectLimitIncrease(limit);
    }

    // Does nothing. It is called on login screen, and we just subscribe to this action in dashboard wrappers.
    function login(): void { }

    function clear() {
        state.user = new User();
        state.settings = DEFAULT_USER_SETTINGS;
        state.userMFASecret = '';
        state.userMFARecoveryCodes = [];
    }

    return {
        state,
        userName,
        shouldOnboard,
        noticeDismissal,
        invalidateSession,
        getSessions,
        setSessionsSortingBy,
        setSessionsSortingDirection,
        updateUser,
        changeEmail,
        deleteAccount,
        getUser,
        disableUserMFA,
        enableUserMFA,
        generateUserMFASecret,
        generateUserMFARecoveryCodes,
        regenerateUserMFARecoveryCodes,
        getShouldPromptPassphrase,
        clear,
        login,
        setUser,
        updateSettings,
        getSettings,
        requestProjectLimitIncrease,
    };
});
