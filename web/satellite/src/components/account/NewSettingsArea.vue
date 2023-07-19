// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="settings">
        <h1 class="settings__title" aria-roledescription="title">Account Settings</h1>

        <div class="settings__section">
            <p class="settings__section__title">Profile</p>
            <div class="settings__section__content">
                <div class="settings__section__content__row">
                    <span class="settings__section__content__row__title">Full Name</span>
                    <span class="settings__section__content__row__subtitle">{{ user.fullName }}</span>
                    <div class="settings__section__content__row__actions">
                        <VButton
                            class="button"
                            font-size="14px"
                            width="110px"
                            is-white
                            :on-press="toggleEditProfileModal"
                            label="Change Name"
                        />
                    </div>
                </div>

                <div class="settings__section__content__row">
                    <span class="settings__section__content__row__title">Email</span>
                    <span class="settings__section__content__row__subtitle">{{ user.email }}</span>
                    <div class="settings__section__content__row__empty-actions" />
                </div>
            </div>
        </div>

        <div class="settings__section">
            <p class="settings__section__title">Security</p>
            <div class="settings__section__content">
                <div class="settings__section__content__row">
                    <span class="settings__section__content__row__title">Password</span>
                    <span class="settings__section__content__row__subtitle">**************</span>
                    <div class="settings__section__content__row__actions">
                        <VButton
                            class="button"
                            is-white
                            font-size="14px"
                            width="136px"
                            :on-press="toggleChangePasswordModal"
                            label="Change Password"
                        />
                    </div>
                </div>

                <div class="settings__section__content__row">
                    <div class="settings__section__content__row__title">
                        <p>Two-Factor</p>
                        <p>Authentication</p>
                    </div>
                    <span v-if="!user.isMFAEnabled" class="settings__section__content__row__subtitle">Improve account security by enabling 2FA.</span>
                    <span v-else class="settings__section__content__row__subtitle">2FA is enabled.</span>
                    <div class="settings__section__content__row__actions">
                        <VButton
                            v-if="user.isMFAEnabled"
                            class="button"
                            font-size="14px"
                            width="208px"
                            label="Regenerate Recovery Codes"
                            is-white
                            :on-press="generateNewMFARecoveryCodes"
                            :is-disabled="isLoading"
                        />
                        <VButton
                            class="button"
                            font-size="14px"
                            width="90px"
                            :is-white="user.isMFAEnabled"
                            :on-press="!user.isMFAEnabled ? enableMFA : toggleDisableMFAModal"
                            :label="!user.isMFAEnabled ? 'Enable 2FA' : 'Disable 2FA'"
                        />
                    </div>
                </div>

                <div class="settings__section__content__row">
                    <span class="settings__section__content__row__title">Session Timeout</span>
                    <span v-if="userDuration" class="settings__section__content__row__subtitle">{{ userDuration.shortString }} of inactivity will log you out.</span>
                    <span v-else class="settings__section__content__row__subtitle">Duration of inactivity that will log you out.</span>
                    <div class="settings__section__content__row__actions">
                        <VButton
                            class="button"
                            is-white
                            font-size="14px"
                            width="100px"
                            :on-press="toggleEditSessionTimeoutModal"
                            label="Set Timeout"
                        />
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue';

import { User } from '@/types/users';
import { AnalyticsErrorEventSource } from '@/utils/constants/analyticsEventNames';
import { MODALS } from '@/utils/constants/appStatePopUps';
import { useNotify } from '@/utils/hooks';
import { useLoading } from '@/composables/useLoading';
import { Duration } from '@/utils/time';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';
import { useProjectsStore } from '@/store/modules/projectsStore';

import VButton from '@/components/common/VButton.vue';

const appStore = useAppStore();
const usersStore = useUsersStore();
const projectsStore = useProjectsStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

/**
 * Returns user info from store.
 */
const user = computed((): User => {
    return usersStore.state.user;
});

/**
 * Returns duration from store.
 */
const userDuration = computed((): Duration | null => {
    return usersStore.state.settings.sessionDuration;
});

/**
 * Toggles enable MFA modal visibility.
 */
function toggleEnableMFAModal(): void {
    appStore.updateActiveModal(MODALS.enableMFA);
}

/**
 * Toggles disable MFA modal visibility.
 */
function toggleDisableMFAModal(): void {
    appStore.updateActiveModal(MODALS.disableMFA);
}

/**
 * Toggles MFA recovery codes modal visibility.
 */
function toggleMFACodesModal(): void {
    appStore.updateActiveModal(MODALS.mfaRecovery);
}

/**
 * Opens change password popup.
 */
function toggleChangePasswordModal(): void {
    appStore.updateActiveModal(MODALS.changePassword);
}

/**
 * Opens edit session timeout modal.
 */
function toggleEditSessionTimeoutModal(): void {
    appStore.updateActiveModal(MODALS.editSessionTimeout);
}

/**
 * Opens edit account info modal.
 */
function toggleEditProfileModal(): void {
    appStore.updateActiveModal(MODALS.editProfile);
}

/**
 * Generates user's MFA secret and opens popup.
 */
async function enableMFA(): Promise<void> {
    await withLoading(async () => {
        try {
            await usersStore.generateUserMFASecret();
            toggleEnableMFAModal();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETTINGS_AREA);
        }
    });
}

/**
 * Toggles generate new MFA recovery codes popup visibility.
 */
async function generateNewMFARecoveryCodes(): Promise<void> {
    await withLoading(async () => {
        try {
            await usersStore.generateUserMFARecoveryCodes();
            toggleMFACodesModal();
        } catch (error) {
            notify.notifyError(error, AnalyticsErrorEventSource.ACCOUNT_SETTINGS_AREA);
        }
    });
}

/**
 * Lifecycle hook after initial render where user info is fetching.
 */
onMounted(() => {
    usersStore.getUser();
});
</script>

<style scoped lang="scss">
.settings {
    padding-bottom: 35px;

    &__title {
        font-family: 'font_bold', sans-serif;
    }

    &__section {
        margin-top: 40px;

        &__title {
            font-family: 'font_medium', sans-serif;
            font-size: 18px;
            line-height: 27px;
        }

        &__content {
            margin-top: 20px;

            &__row {
                background: var(--c-white);
                box-shadow: 0 0 20px rgb(0 0 0 / 4%);
                border-radius: 8px;
                padding: 10px 30px;
                margin-bottom: 20px;
                height: 88px;
                box-sizing: border-box;
                display: grid;
                grid-template-columns: 1fr 1fr 1fr;
                align-items: center;

                @media screen and (width <= 500px) {
                    display: flex;
                    flex-direction: column;
                    align-items: flex-start;
                    justify-content: center;
                    gap: 10px;
                    height: unset;
                }

                &__title {
                    font-family: 'font_medium', sans-serif;
                    font-size: 14px;
                    line-height: 20px;
                }

                &__subtitle {
                    font-family: 'font_regular', sans-serif;
                    font-size: 14px;
                    line-height: 20px;
                    color: var(--c-grey-6);
                    word-wrap: break-word;
                    overflow: hidden;

                    @media screen and (width <= 500px) {
                        width: 100%;
                    }
                }

                &__actions {
                    display: flex;
                    align-items: center;
                    justify-content: flex-end;
                    gap: 5px;

                    @media screen and (width <= 500px) {
                        width: 100%;
                        justify-content: flex-start;
                    }
                }
            }
        }
    }
}

.button {
    padding: 5px;
}
</style>
