// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="edit-profile-popup-container">
        <div class="edit-profile-popup">
            <div class="edit-profile-popup__form-container">
                <div class="edit-profile-row-container">
                    <div class="edit-profile-popup__form-container__avatar">
                        <h1 class="edit-profile-popup__form-container__avatar__letter">{{avatarLetter}}</h1>
                    </div>
                    <h2 class="edit-profile-popup__form-container__main-label-text">Edit Profile</h2>
                </div>
                <HeaderedInput
                    class="full-input"
                    label="Full Name"
                    placeholder="Enter Full Name"
                    width="100%"
                    ref="fullNameInput"
                    :error="fullNameError"
                    :init-value="userInfo.fullName"
                    @setData="setFullName"
                />
                <HeaderedInput
                    class="full-input"
                    label="Nickname"
                    placeholder="Enter Nickname"
                    width="100%"
                    ref="shortNameInput"
                    :init-value="userInfo.shortName"
                    @setData="setShortName"
                />
                <div class="edit-profile-popup__form-container__button-container">
                    <VButton
                        label="Cancel"
                        width="205px"
                        height="48px"
                        :on-press="onCloseClick"
                        is-transparent="true"
                    />
                    <VButton
                        label="Update"
                        width="205px"
                        height="48px"
                        :on-press="onUpdateClick"
                    />
                </div>
            </div>
            <div class="edit-profile-popup__close-cross-container" @click="onCloseClick">
                <CloseCrossIcon/>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderedInput from '@/components/common/HeaderedInput.vue';
import VButton from '@/components/common/VButton.vue';

import CloseCrossIcon from '@/../static/images/common/closeCross.svg';

import { USER_ACTIONS } from '@/store/modules/users';
import { UpdatedUser } from '@/types/users';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';

@Component({
    components: {
        CloseCrossIcon,
        HeaderedInput,
        VButton,
    },
})
export default class EditProfilePopup extends Vue {
    private fullNameError: string = '';

    private readonly userInfo: UpdatedUser =
        new UpdatedUser(this.$store.getters.user.fullName, this.$store.getters.user.shortName);

    public setFullName(value: string): void {
        this.userInfo.setFullName(value);
        this.fullNameError = '';
    }

    public setShortName(value: string): void {
        this.userInfo.setShortName(value);
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

        await this.$notify.success('Account info successfully updated!');

        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_EDIT_PROFILE_POPUP);
    }

    /**
     * Closes popup.
     */
    public onCloseClick(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_EDIT_PROFILE_POPUP);
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
    .edit-profile-row-container {
        width: 100%;
        display: flex;
        flex-direction: row;
        align-content: center;
        justify-content: flex-start;
    }

    .edit-profile-popup-container {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(134, 134, 148, 0.4);
        z-index: 1000;
        display: flex;
        justify-content: center;
        align-items: center;
        font-family: 'font_regular', sans-serif;
    }

    .input-container.full-input {
        width: 100%;
    }

    .edit-profile-popup {
        width: 100%;
        max-width: 440px;
        background-color: #fff;
        border-radius: 6px;
        display: flex;
        flex-direction: row;
        align-items: flex-start;
        position: relative;
        justify-content: center;
        padding: 80px;

        &__info-panel-container {
            display: flex;
            flex-direction: column;
            justify-content: flex-start;
            align-items: center;
            margin-right: 100px;
            margin-top: 20px;
        }

        &__form-container {
            width: 100%;
            max-width: 440px;
            margin-top: 10px;

            &__avatar {
                width: 60px;
                height: 60px;
                border-radius: 6px;
                display: flex;
                align-items: center;
                justify-content: center;
                background: #e8eaf2;
                margin-right: 20px;

                &__letter {
                    font-family: 'font_medium', sans-serif;
                    font-size: 16px;
                    line-height: 23px;
                    color: #354049;
                }
            }

            &__main-label-text {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 60px;
                color: #384b65;
                margin-top: 0;
            }

            &__button-container {
                width: 100%;
                display: flex;
                flex-direction: row;
                justify-content: space-between;
                align-items: center;
                margin-top: 40px;
            }
        }

        &__close-cross-container {
            display: flex;
            justify-content: center;
            align-items: center;
            position: absolute;
            right: 30px;
            top: 40px;
            height: 24px;
            width: 24px;
            cursor: pointer;

            &:hover .close-cross-svg-path {
                fill: #2683ff;
            }
        }
    }

    @media screen and (max-width: 720px) {

        .edit-profile-popup {

            &__info-panel-container {
                display: none;
            }

            &__form-container {

                &__button-container {
                    width: 100%;
                }
            }
        }
    }
</style>
