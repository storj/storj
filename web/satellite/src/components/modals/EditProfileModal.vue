// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <VModal :on-close="closeModal">
        <template #content>
            <div class="edit-profile">
                <div class="edit-profile__row">
                    <div class="edit-profile__row__avatar">
                        <h1 class="edit-profile__row__avatar__letter">{{ avatarLetter }}</h1>
                    </div>
                    <h2 class="edit-profile__row__label">Edit Profile</h2>
                </div>
                <VInput
                    label="Full Name"
                    placeholder="Enter Full Name"
                    max-symbols="72"
                    :error="fullNameError"
                    :init-value="userInfo.fullName"
                    @setData="setFullName"
                />
                <div class="edit-profile__buttons">
                    <VButton
                        label="Cancel"
                        width="100%"
                        height="48px"
                        :on-press="closeModal"
                        :is-transparent="true"
                    />
                    <VButton
                        label="Update"
                        width="100%"
                        height="48px"
                        :on-press="onUpdateClick"
                    />
                </div>
            </div>
        </template>
    </VModal>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue';

import { UpdatedUser } from '@/types/users';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsErrorEventSource, AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { useNotify } from '@/utils/hooks';
import { useUsersStore } from '@/store/modules/usersStore';
import { useAppStore } from '@/store/modules/appStore';

import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';

const appStore = useAppStore();
const userStore = useUsersStore();
const notify = useNotify();

const analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

const userInfo = reactive<UpdatedUser>(new UpdatedUser(userStore.state.user.fullName, userStore.state.user.shortName));
const fullNameError = ref<string>('');

/**
 * Returns first letter of user name.
 */
const avatarLetter = computed((): string => {
    return userStore.userName.slice(0, 1).toUpperCase();
});

/**
 * Set full name value from input.
 */
function setFullName(value: string): void {
    userInfo.setFullName(value);
    fullNameError.value = '';
}

/**
 * Validates name and tries to update user info and close popup.
 */
async function onUpdateClick(): Promise<void> {
    if (!userInfo.isValid()) {
        fullNameError.value = 'Full name expected';

        return;
    }

    try {
        await userStore.updateUser(userInfo);
    } catch (error) {
        notify.error(error.message, AnalyticsErrorEventSource.EDIT_PROFILE_MODAL);

        return;
    }

    analytics.eventTriggered(AnalyticsEvent.PROFILE_UPDATED);

    notify.success('Account info successfully updated!');

    closeModal();
}

/**
 * Closes modal.
 */
function closeModal(): void {
    appStore.removeActiveModal();
}
</script>

<style scoped lang="scss">
    .edit-profile {
        background-color: #fff;
        border-radius: 6px;
        display: flex;
        flex-direction: column;
        padding: 48px;
        min-width: 530px;
        box-sizing: border-box;

        @media screen and (width <= 580px) {
            min-width: 450px;
        }

        @media screen and (width <= 500px) {
            min-width: unset;
        }

        @media screen and (width <= 400px) {
            padding: 24px;
        }

        &__row {
            display: flex;
            align-items: center;
            margin-bottom: 30px;

            @media screen and (width <= 400px) {
                margin-bottom: 0;
            }

            &__avatar {
                width: 60px;
                height: 60px;
                border-radius: 6px;
                display: flex;
                align-items: center;
                justify-content: center;
                background: #e8eaf2;
                margin-right: 20px;

                @media screen and (width <= 400px) {
                    display: none;
                }

                &__letter {
                    font-family: 'font_medium', sans-serif;
                    font-size: 16px;
                    line-height: 23px;
                    color: #354049;
                }
            }

            &__label {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 60px;
                color: #384b65;
                margin-top: 0;
            }
        }

        &__buttons {
            width: 100%;
            display: flex;
            align-items: center;
            margin-top: 40px;
            column-gap: 20px;

            @media screen and (width <= 400px) {
                flex-direction: column-reverse;
                column-gap: unset;
                row-gap: 10px;
                margin-top: 20px;
            }
        }
    }
</style>
