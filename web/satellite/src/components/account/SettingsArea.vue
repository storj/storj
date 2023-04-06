// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="settings">
        <h1 class="settings__title" aria-roledescription="title">Account Settings</h1>
        <div class="settings__edit-profile">
            <div class="settings__edit-profile__row">
                <div class="settings__edit-profile__avatar">
                    <h1 class="settings__edit-profile__avatar__letter">{{ avatarLetter }}</h1>
                </div>
                <div class="settings__edit-profile__text">
                    <h2 class="profile-bold-text">Edit Profile</h2>
                    <h3 class="profile-regular-text">This information will be visible to all users</h3>
                </div>
            </div>
            <EditIcon
                class="edit-svg"
                @click="toggleEditProfileModal"
            />
        </div>
        <div class="settings__secondary-container">
            <div class="settings__secondary-container__change-password">
                <div class="settings__edit-profile__row">
                    <ChangePasswordIcon class="settings__secondary-container__change-password__img" />
                    <div class="settings__secondary-container__change-password__text-container">
                        <h2 class="profile-bold-text">Change Password</h2>
                        <h3 class="profile-regular-text">6 or more characters</h3>
                    </div>
                </div>
                <EditIcon
                    class="edit-svg"
                    @click="toggleChangePasswordModal"
                />
            </div>
            <div class="settings__secondary-container__email-container">
                <div class="settings__secondary-container__email-container__row">
                    <EmailIcon class="settings__secondary-container__img" />
                    <div class="settings__secondary-container__email-container__text-container">
                        <h2 class="profile-bold-text email">{{ user.email }}</h2>
                    </div>
                </div>
            </div>
        </div>
        <div class="settings__mfa">
            <h2 class="profile-bold-text">Two-Factor Authentication</h2>
            <p v-if="!user.isMFAEnabled" class="profile-regular-text">
                To increase your account security, we strongly recommend enabling 2FA on your account.
            </p>
            <p v-else class="profile-regular-text">
                2FA is enabled.
            </p>
            <div class="settings__mfa__buttons">
                <VButton
                    v-if="!user.isMFAEnabled"
                    label="Enable 2FA"
                    width="173px"
                    height="44px"
                    :on-press="enableMFA"
                    :is-disabled="isLoading"
                />
                <div v-else class="settings__mfa__buttons__row">
                    <VButton
                        class="margin-right"
                        label="Disable 2FA"
                        width="173px"
                        height="44px"
                        :on-press="toggleDisableMFAModal"
                        :is-deletion="true"
                    />
                    <VButton
                        label="Regenerate Recovery Codes"
                        width="240px"
                        height="44px"
                        :on-press="generateNewMFARecoveryCodes"
                        :is-blue-white="true"
                        :is-disabled="isLoading"
                    />
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
import { useNotify, useStore } from '@/utils/hooks';
import { useLoading } from '@/composables/useLoading';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';
import { useUsersStore } from '@/store/modules/usersStore';

import VButton from '@/components/common/VButton.vue';

import ChangePasswordIcon from '@/../static/images/account/profile/changePassword.svg';
import EmailIcon from '@/../static/images/account/profile/email.svg';
import EditIcon from '@/../static/images/common/edit.svg';

const usersStore = useUsersStore();
const store = useStore();
const notify = useNotify();
const { isLoading, withLoading } = useLoading();

/**
 * Returns user info from store.
 */
const user = computed((): User => {
    return usersStore.state.user;
});

/**
 * Returns first letter of user name.
 */
const avatarLetter = computed((): string => {
    return usersStore.userName.slice(0, 1).toUpperCase();
});

/**
 * Toggles enable MFA modal visibility.
 */
function toggleEnableMFAModal(): void {
    store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.enableMFA);
}

/**
 * Toggles disable MFA modal visibility.
 */
function toggleDisableMFAModal(): void {
    store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.disableMFA);
}

/**
 * Toggles MFA recovery codes modal visibility.
 */
function toggleMFACodesModal(): void {
    store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.mfaRecovery);
}

/**
 * Opens change password popup.
 */
function toggleChangePasswordModal(): void {
    store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.changePassword);
}

/**
 * Opens edit account info modal.
 */
function toggleEditProfileModal(): void {
    store.commit(APP_STATE_MUTATIONS.UPDATE_ACTIVE_MODAL, MODALS.editProfile);
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
            await notify.error(error.message, AnalyticsErrorEventSource.ACCOUNT_SETTINGS_AREA);
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
            await notify.error(error.message, AnalyticsErrorEventSource.ACCOUNT_SETTINGS_AREA);
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
    h3 {
        margin-block-start: 0;
        margin-block-end: 0;
    }

    .settings {
        position: relative;
        font-family: 'font_regular', sans-serif;
        padding-bottom: 70px;

        &__title {
            font-family: 'font_bold', sans-serif;
            font-size: 32px;
            line-height: 39px;
            color: #263549;
            margin: 40px 0 25px;
        }

        &__edit-profile {
            height: 137px;
            box-sizing: border-box;
            width: 100%;
            border-radius: 6px;
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 37px 40px;
            background-color: #fff;

            &__row {
                display: flex;
                justify-content: flex-start;
                align-items: center;
                max-width: calc(100% - 40px);
            }

            &__avatar {
                width: 60px;
                height: 60px;
                border-radius: 6px;
                display: flex;
                align-items: center;
                justify-content: center;
                background: #e8eaf2;
                margin-right: 32px;

                &__letter {
                    font-family: 'font_medium', sans-serif;
                    font-size: 16px;
                    line-height: 23px;
                    color: #354049;
                }
            }
        }

        &__secondary-container {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-top: 40px;

            &__change-password {
                height: 137px;
                border-radius: 6px;
                display: flex;
                justify-content: space-between;
                align-items: center;
                padding: 37px 40px;
                background-color: #fff;
                width: 48%;
                box-sizing: border-box;

                &__text-container {
                    margin-left: 32px;
                }
            }

            &__email-container {
                height: 137px;
                border-radius: 6px;
                display: flex;
                justify-content: flex-start;
                align-items: center;
                padding: 37px 40px;
                background-color: #fff;
                width: 48%;
                box-sizing: border-box;

                &__row {
                    display: flex;
                    justify-content: flex-start;
                    align-items: center;
                    width: 100%;
                }

                &__text-container {
                    margin-left: 32px;
                }
            }

            &__img {
                min-width: 60px;
                min-height: 60px;
            }
        }

        &__mfa {
            margin-top: 40px;
            padding: 40px;
            border-radius: 6px;
            background-color: #fff;

            &__buttons {
                margin-top: 20px;

                &__row {
                    display: flex;
                    align-items: center;
                }
            }
        }
    }

    .margin-right {
        margin-right: 15px;
    }

    .edit-svg {
        cursor: pointer;

        &:hover {

            .edit-svg__rect {
                fill: #2683ff;
            }

            .edit-svg__path {
                fill: white;
            }
        }
    }

    .input-container.full-input,
    .input-wrap.full-input {
        width: 100%;
    }

    .profile-bold-text {
        font-family: 'font_bold', sans-serif;
        color: #354049;
        font-size: 18px;
        line-height: 27px;
    }

    .profile-regular-text {
        margin: 10px 0;
        color: #afb7c1;
        font-size: 16px;
        line-height: 21px;
    }

    .email {
        word-break: break-all;
    }

    @media screen and (max-width: 1300px) {

        .settings {

            &__secondary-container {
                flex-direction: column;
                justify-content: center;

                &__change-password,
                &__email-container {
                    height: auto;
                    width: 100%;
                }

                &__email-container {
                    margin-top: 40px;
                }
            }
        }
    }

    @media screen and (max-height: 825px) {

        .settings {
            height: 535px;
            overflow-y: scroll;

            &__secondary-container {
                margin-top: 20px;

                &__email-container {
                    margin-top: 20px;
                }
            }

            &__button-area {
                margin-top: 20px;
            }
        }
    }

    @media screen and (max-width: 650px) {

        .settings__secondary-container__change-password__text-container {
            margin: 0;
        }

        .settings__edit-profile__avatar,
        .settings__secondary-container__change-password__img {
            display: none;
        }
    }

    @media screen and (max-width: 460px) {

        .settings__edit-profile,
        .settings__secondary-container__change-password,
        .settings__secondary-container__email-container {
            padding: 25px;
        }
    }
</style>
