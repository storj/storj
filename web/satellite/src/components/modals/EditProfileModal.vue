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
                        is-transparent="true"
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

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import { USER_ACTIONS } from '@/store/modules/users';
import { UpdatedUser } from '@/types/users';
import { AnalyticsHttpApi } from '@/api/analytics';
import { AnalyticsEvent } from '@/utils/constants/analyticsEventNames';
import { APP_STATE_MUTATIONS } from '@/store/mutationConstants';

import VModal from '@/components/common/VModal.vue';
import VButton from '@/components/common/VButton.vue';
import VInput from '@/components/common/VInput.vue';

// @vue/component
@Component({
    components: {
        VInput,
        VButton,
        VModal,
    },
})
export default class EditProfileModal extends Vue {
    private fullNameError = '';

    private readonly userInfo: UpdatedUser =
        new UpdatedUser(this.$store.getters.user.fullName, this.$store.getters.user.shortName);

    private readonly analytics: AnalyticsHttpApi = new AnalyticsHttpApi();

    /**
     * Set full name value from input.
     */
    public setFullName(value: string): void {
        this.userInfo.setFullName(value);
        this.fullNameError = '';
    }

    /**
     * Validates name and tries to update user info and close popup.
     */
    public async onUpdateClick(): Promise<void> {
        if (!this.userInfo.isValid()) {
            this.fullNameError = 'Full name expected';

            return;
        }

        try {
            await this.$store.dispatch(USER_ACTIONS.UPDATE, this.userInfo);
        } catch (error) {
            await this.$notify.error(error.message);

            return;
        }

        this.analytics.eventTriggered(AnalyticsEvent.PROFILE_UPDATED);

        await this.$notify.success('Account info successfully updated!');

        this.closeModal();
    }

    /**
     * Closes modal.
     */
    public closeModal(): void {
        this.$store.commit(APP_STATE_MUTATIONS.TOGGLE_EDIT_PROFILE_MODAL_SHOWN);
    }

    /**
     * Returns first letter of user name.
     */
    public get avatarLetter(): string {
        return this.$store.getters.userName.slice(0, 1).toUpperCase();
    }
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

        @media screen and (max-width: 580px) {
            min-width: 450px;
        }

        @media screen and (max-width: 500px) {
            min-width: unset;
        }

        @media screen and (max-width: 400px) {
            padding: 24px;
        }

        &__row {
            display: flex;
            align-items: center;
            margin-bottom: 30px;

            @media screen and (max-width: 400px) {
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

                @media screen and (max-width: 400px) {
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

            @media screen and (max-width: 400px) {
                flex-direction: column-reverse;
                column-gap: unset;
                row-gap: 10px;
                margin-top: 20px;
            }
        }
    }
</style>
