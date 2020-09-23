// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="change-password-popup-container">
        <div class="change-password-popup">
            <div class="change-password-popup__form-container">
                <div class="change-password-row-container">
                    <ChangePasswordIcon class="change-password-popup__form-container__svg"/>
                    <h2 class="change-password-popup__form-container__main-label-text">Change Password</h2>
                </div>
                <HeaderlessInput
                    class="full-input"
                    label="Old Password"
                    placeholder ="Enter Old Password"
                    width="100%"
                    is-password="true"
                    ref="oldPasswordInput"
                    :error="oldPasswordError"
                    @setData="setOldPassword"
                />
                <div class="password-input">
                    <HeaderlessInput
                        class="full-input"
                        label="New Password"
                        placeholder ="Enter New Password"
                        width="100%"
                        ref="newPasswordInput"
                        is-password="true"
                        :error="newPasswordError"
                        @setData="setNewPassword"
                        @showPasswordStrength="showPasswordStrength"
                        @hidePasswordStrength="hidePasswordStrength"
                    />
                    <PasswordStrength
                        :password-string="newPassword"
                        :is-shown="isPasswordStrengthShown"
                    />
                </div>
                <HeaderlessInput
                    class="full-input"
                    label="Confirm Password"
                    placeholder="Confirm Password"
                    width="100%"
                    ref="confirmPasswordInput"
                    is-password="true"
                    :error="confirmationPasswordError"
                    @setData="setPasswordConfirmation"
                />
                <div class="change-password-popup__form-container__button-container">
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
            <div class="change-password-popup__close-cross-container" @click="onCloseClick">
                <CloseCrossIcon/>
            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';

import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import PasswordStrength from '@/components/common/PasswordStrength.vue';
import VButton from '@/components/common/VButton.vue';

import ChangePasswordIcon from '@/../static/images/account/changePasswordPopup/changePassword.svg';
import CloseCrossIcon from '@/../static/images/common/closeCross.svg';

import { AuthHttpApi } from '@/api/auth';
import { APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import { Validator } from '@/utils/validation';

@Component({
    components: {
        ChangePasswordIcon,
        CloseCrossIcon,
        HeaderlessInput,
        VButton,
        PasswordStrength,
    },
})
export default class ChangePasswordPopup extends Vue {
    private oldPassword: string = '';
    private newPassword: string = '';
    private confirmationPassword: string = '';

    private oldPasswordError: string = '';
    private newPasswordError: string = '';
    private confirmationPasswordError: string = '';

    private readonly auth: AuthHttpApi = new AuthHttpApi();

    /**
     * Indicates if hint popup needs to be shown while creating new password.
     */
    public isPasswordStrengthShown: boolean = false;

    public showPasswordStrength(): void {
        this.isPasswordStrengthShown = true;
    }

    public hidePasswordStrength(): void {
        this.isPasswordStrengthShown = false;
    }

    public setOldPassword(value: string): void {
        this.oldPassword = value;
        this.oldPasswordError = '';
    }

    public setNewPassword(value: string): void {
        this.newPassword = value;
        this.newPasswordError = '';
    }

    public setPasswordConfirmation(value: string): void {
        this.confirmationPassword = value;
        this.confirmationPasswordError = '';
    }

    /**
     * Validates inputs and if everything are correct tries to change password and close popup.
     */
    public async onUpdateClick(): Promise<void> {
        let hasError = false;
        if (this.oldPassword.length < 6) {
            this.oldPasswordError = 'Invalid old password. Must be 6 or more characters';
            hasError = true;
        }

        if (!Validator.password(this.newPassword)) {
            this.newPasswordError = 'Invalid password. Use 6 or more characters';
            hasError = true;
        }

        if (!this.confirmationPassword) {
            this.confirmationPasswordError = 'Password required';
            hasError = true;
        }

        if (this.newPassword !== this.confirmationPassword) {
            this.confirmationPasswordError = 'Password not match to new one';
            hasError = true;
        }

        if (hasError) {
            return;
        }

        try {
            await this.auth.changePassword(this.oldPassword, this.newPassword);
        } catch (error) {
            await this.$notify.error(error.message);

            return;
        }

        await this.$notify.success('Password successfully changed!');
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_CHANGE_PASSWORD_POPUP);
    }

    /**
     * Closes popup.
     */
    public onCloseClick(): void {
        this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_CHANGE_PASSWORD_POPUP);
    }
}
</script>

<style scoped lang="scss">
    .change-password-popup-container {
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

    .change-password-row-container {
        width: 100%;
        display: flex;
        flex-direction: row;
        align-content: center;
        justify-content: flex-start;
        margin-bottom: 20px;
    }

    .change-password-popup {
        width: 100%;
        max-width: 440px;
        max-height: 470px;
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

            &__main-label-text {
                font-family: 'font_bold', sans-serif;
                font-size: 32px;
                line-height: 60px;
                color: #384b65;
                margin-bottom: 0;
                margin-top: 0;
                margin-left: 32px;
            }

            &__button-container {
                width: 100%;
                display: flex;
                flex-direction: row;
                justify-content: space-between;
                align-items: center;
                margin-top: 32px;
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

    .password-input {
        position: relative;
        width: 100%;
    }

    @media screen and (max-width: 720px) {

        .change-password-popup {

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
