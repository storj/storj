// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-area-container">
        <div class="account-area-row-container__main">
            <!--start of Account settings area -->
            <div class="account-area-settings-container">
                <div class="account-area-row-container">
                    <div class="account-area-settings-container__avatar">
                        <h1>{{avatarLetter}}</h1>
                    </div>
                    <div>
                        <h1>Account Settings</h1>
                        <h2>This information will be visible to all users</h2>
                    </div>
                </div>
                <HeaderedInput
                    class="full-input"
                    label="Full name"
                    placeholder="Enter Full Name"
                    width="100%"
                    ref="fullNameInput"
                    :error="fullNameError"
                    :initValue="user.fullName"
                    @setData="setFullName" />
                <HeaderedInput
                    class="full-input"
                    label="Short Name"
                    placeholder="Enter Short Name"
                    width="100%"
                    ref="shortNameInput"
                    :initValue="user.shortName"
                    @setData="setShortName"/>
                <HeaderedInput
                    class="full-input"
                    label="Email"
                    placeholder ="Enter Email"
                    width="100%"
                    ref="emailInput"
                    :error="emailError"
                    :initValue="user.email"
                    @setData="setEmail" />
                <div v-if="isAccountSettingsEditing" class="account-area-save-button-area" >
                    <div class="account-area-save-button-area__btn-active">
                        <Button class="account-area-save-button-area__cancel-button" label="Cancel" width="288px"  height="56px" :onPress="cancelAccountSettings" isWhite/>
                        <Button class="account-area-save-button-area__save-button" label="Save" width="288px"  height="56px" :onPress="onSaveAccountSettingsButtonClick"/>
                    </div>
                </div>
                <div v-if="!isAccountSettingsEditing" class="account-area-save-button-area" >
                    <div class="account-area-save-button-area__btn">
                        <Button class="account-area-save-button-area__save-button" label="Save" width="288px"  height="56px" :onPress="onSaveAccountSettingsButtonClick" isDisabled />
                    </div>
                </div>
            </div>
            <!--end of Account settings area -->
            <!--start of Password area -->
            <div class="account-area-password-container">
                <div class="account-area-row-container">
                    <svg class="account-area-password-container__svg" width="60" height="60" viewBox="0 0 60 60" fill="none" xmlns="http://www.w3.org/2000/svg">
                        <path d="M30 60C46.5685 60 60 46.5685 60 30C60 13.4315 46.5685 0 30 0C13.4315 0 0 13.4315 0 30C0 46.5685 13.4315 60 30 60Z" fill="#2683FF"/>
                        <path d="M29.5001 34.5196C30.1001 34.5196 30.5865 34.0452 30.5865 33.46C30.5865 32.8748 30.1001 32.4004 29.5001 32.4004C28.9 32.4004 28.4136 32.8748 28.4136 33.46C28.4136 34.0452 28.9 34.5196 29.5001 34.5196Z" fill="#FEFEFF"/>
                        <path d="M39.9405 40.2152C40.1781 40 40.3139 39.6854 40.3139 39.3709V25.5464C40.3139 24.9007 39.7707 24.3709 39.1086 24.3709H35.7473V21.0927C35.7473 17.7318 32.9462 15 29.5 15C26.0538 15 23.2527 17.7318 23.2527 21.0927V24.3709H19.8914C19.2293 24.3709 18.686 24.9007 18.686 25.5464V39.3709C18.686 39.6854 18.8218 40 19.0595 40.2152L23.7959 44.6689C24.0166 44.8841 24.3222 45 24.6278 45H34.3552C34.6608 45 34.9664 44.8841 35.1871 44.6689L39.9405 40.2152ZM30.7053 36.6391V38.1291C30.7053 38.7748 30.1621 39.3046 29.5 39.3046C28.8379 39.3046 28.2947 38.7748 28.2947 38.1291V36.6391C26.9705 36.1589 26.0198 34.9172 26.0198 33.4437C26.0198 31.5728 27.5817 30.0497 29.5 30.0497C31.4183 30.0497 32.9801 31.5728 32.9801 33.4437C32.9801 34.9172 32.0295 36.1589 30.7053 36.6391ZM33.3367 24.3709H25.6464V21.0927C25.6464 19.0232 27.3779 17.351 29.483 17.351C31.5881 17.351 33.3197 19.0397 33.3197 21.0927V24.3709H33.3367Z" fill="#FEFEFF"/>
                        <defs>
                            <clipPath id="clip0">
                                <rect width="21.6279" height="30" fill="#FFFFFF" transform="translate(18.686 15)"/>
                            </clipPath>
                        </defs>
                    </svg>
                    <div>
                        <h1>Change Password</h1>
                        <h2>6 or more characters, at least 1 letter and number.</h2>
                    </div>
                </div>
                <HeaderlessInput
                    class="full-input"
                    label="Old Password"
                    placeholder ="Enter Old Password"
                    width="100%"
                    isPassword
                    ref="oldPasswordInput"
                    :error="oldPasswordError"
                    @setData="setOldPassword" />
                <HeaderlessInput
                    class="full-input mt"
                    label="New Password"
                    placeholder ="Enter New Password"
                    width="100%"
                    ref="newPasswordInput"
                    isPassword
                    :error="newPasswordError"
                    @setData="setNewPassword" />
                <HeaderlessInput
                    class="full-input mt"
                    label="Confirm password"
                    placeholder="Confirm password"
                    width="100%"
                    ref="confirmPasswordInput"
                    isPassword
                    :error="confirmationPasswordError"
                    @setData="setPasswordConfirmation" />
                <div v-if="isPasswordEditing" class="account-area-save-button-area active" >
                    <div class="account-area-save-button-area__btn-active">
                        <Button class="account-area-save-button-area__cancel-button" label="Cancel" width="288px" height="56px" :onPress="onCancelPasswordEditButtonClick" isWhite/>
                        <Button class="account-area-save-button-area__save-button" label="Save" width="288px" height="56px" :onPress="onSavePasswordButtonClick"/>
                    </div>
                </div>
                <div v-if="!isPasswordEditing" class="account-area-save-button-area" >
                    <div class="account-area-save-button-area__btn">
                        <Button class="account-area-save-button-area__save-button" label="Save" width="288px" height="56px" isDisabled/>
                    </div>
                </div>
            </div>
            <!--end of Password area -->
        </div>
        <div class="account-area-button-area" id="deleteAccountPopupButton">
            <Button class="account-area-save-button-area__delete-button" label="Delete account" width="210px" height="56px" :onPress="togglePopup" isDeletion/>
        </div>
        <DeleteAccountPopup v-if="isPopupShown" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import Button from '@/components/common/Button.vue';
import HeaderedInput from '@/components/common/HeaderedInput.vue';
import HeaderlessInput from '@/components/common/HeaderlessInput.vue';
import Checkbox from '@/components/common/Checkbox.vue';
import { USER_ACTIONS, NOTIFICATION_ACTIONS, APP_STATE_ACTIONS } from '@/utils/constants/actionNames';
import DeleteAccountPopup from '@/components/account/DeleteAccountPopup.vue';
import { validateEmail, validatePassword } from '@/utils/validation';

@Component(
    {
        data: function () {
            return {
                originalFullName: this.$store.getters.user.fullName,
                originalShortName: this.$store.getters.user.shortName,
                originalEmail: this.$store.getters.user.email,

                fullName: this.$store.getters.user.fullName,
                shortName: this.$store.getters.user.shortName,
                email: this.$store.getters.user.email,

                fullNameError: '',
                emailError: '',

                isAccountSettingsEditing: false,

                oldPassword: '',
                newPassword: '',
                confirmationPassword: '',

                oldPasswordError: '',
                newPasswordError: '',
                confirmationPasswordError: '',
                isPasswordEditing: false,
            };
        },
        methods: {
            setFullName: function (value: string) {
                this.$data.fullName = value;
                this.$data.fullNameError = '';
                this.$data.isAccountSettingsEditing = true;
            },
            setShortName: function (value: string) {
                this.$data.shortName = value;
                this.$data.isAccountSettingsEditing = true;
            },
            setEmail: function (value: string) {
                this.$data.email = value;
                this.$data.emailError = '';
                this.$data.isAccountSettingsEditing = true;
            },
            cancelAccountSettings: function () {
                this.$data.fullName = this.$data.originalFullName;
                this.$data.fullNameError = '';
                this.$data.shortName = this.$data.originalShortName;
                this.$data.email = this.$data.originalEmail;
                this.$data.emailError = '';

                let fullNameInput: any = this.$refs['fullNameInput'];
                fullNameInput.setValue(this.$data.originalFullName);

                let shortNameInput: any = this.$refs['shortNameInput'];
                shortNameInput.setValue(this.$data.originalShortName);

                let emailInput: any = this.$refs['emailInput'];
                emailInput.setValue(this.$data.originalEmail);

                this.$data.isAccountSettingsEditing = false;
            },
            onSaveAccountSettingsButtonClick: async function () {
                let hasError = false;

                if (!this.$data.fullName) {
                    this.$data.fullNameError = 'Full name expected';
                    hasError = true;
                }

                if (!validateEmail(this.$data.email)) {
                    this.$data.emailError = 'Incorrect email';
                    hasError = true;
                }

                if (hasError) {
                    return;
                }

                let user = {
                    email: this.$data.email,
                    fullName: this.$data.fullName,
                    shortName: this.$data.shortName,
                };

                let response = await this.$store.dispatch(USER_ACTIONS.UPDATE, user);
                if (!response.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);

                    return;
                }

                this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Account info successfully updated!');

                this.$data.originalFullName = this.$store.getters.user.fullName;
                this.$data.originalShortName = this.$store.getters.user.shortName;
                this.$data.originalEmail = this.$store.getters.user.email;

                this.$data.isAccountSettingsEditing = false;
            },

            setOldPassword: function (value: string) {
                this.$data.oldPassword = value;
                this.$data.oldPasswordError = '';
                this.$data.isPasswordEditing = true;
            },
            setNewPassword: function (value: string) {
                this.$data.newPassword = value;
                this.$data.newPasswordError = '';
                this.$data.isPasswordEditing = true;
            },
            setPasswordConfirmation: function (value: string) {
                this.$data.confirmationPassword = value;
                this.$data.confirmationPasswordError = '';
                this.$data.isPasswordEditing = true;
            },
            onCancelPasswordEditButtonClick: function () {
                this.$data.oldPassword = '';
                this.$data.newPassword = '';
                this.$data.confirmationPassword = '';

                this.$data.oldPasswordError = '';
                this.$data.newPasswordError = '';
                this.$data.confirmationPasswordError = '';

                let oldPasswordInput: any = this.$refs['oldPasswordInput'];
                oldPasswordInput.setValue('');

                let newPasswordInput: any = this.$refs['newPasswordInput'];
                newPasswordInput.setValue('');

                let confirmPasswordInput: any = this.$refs['confirmPasswordInput'];
                confirmPasswordInput.setValue('');

                this.$data.isPasswordEditing = false;
            },
            onSavePasswordButtonClick: async function () {
                let hasError = false;

                if (!this.$data.oldPassword) {
                    this.$data.oldPasswordError = 'Password required';
                    hasError = true;
                }

                if (!validatePassword(this.$data.newPassword)) {
                    this.$data.newPasswordError = 'Invalid password';
                    hasError = true;
                }

                if (!this.$data.confirmationPassword) {
                    this.$data.confirmationPasswordError = 'Password required';
                    hasError = true;
                }

                if (this.$data.newPassword !== this.$data.confirmationPassword) {
                    this.$data.confirmationPasswordError = 'Password not match to new one';
                    hasError = true;
                }

                if (hasError) {
                    return;
                }

                let response = await this.$store.dispatch(USER_ACTIONS.CHANGE_PASSWORD,
                    {
                        oldPassword: this.$data.oldPassword,
                        newPassword: this.$data.newPassword
                    }
                );
                if (!response.isSuccess) {
                    this.$store.dispatch(NOTIFICATION_ACTIONS.ERROR, response.errorMessage);

                    return;
                }

                this.$store.dispatch(NOTIFICATION_ACTIONS.SUCCESS, 'Password successfully changed!');

                this.$data.oldPassword = '';
                this.$data.newPassword = '';
                this.$data.confirmationPassword = '';

                this.$data.oldPasswordError = '';
                this.$data.newPasswordError = '';
                this.$data.confirmationPasswordError = '';

                let oldPasswordInput: any = this.$refs['oldPasswordInput'];
                oldPasswordInput.setValue('');

                let newPasswordInput: any = this.$refs['newPasswordInput'];
                newPasswordInput.setValue('');

                let confirmPasswordInput: any = this.$refs['confirmPasswordInput'];
                confirmPasswordInput.setValue('');

                this.$data.isPasswordEditing = false;
            },
            togglePopup: function(): void {
                this.$store.dispatch(APP_STATE_ACTIONS.TOGGLE_DEL_ACCOUNT);
            },
        },
        computed: {
            user: function() {
                return {
                    fullName: this.$store.getters.user.fullName,
                    shortName: this.$store.getters.user.shortName,
                    email: this.$store.getters.user.email,
                };
            },
            // May change later
            avatarLetter: function (): string {
                return this.$store.getters.userName.slice(0, 1).toUpperCase();
            },
            isPopupShown: function (): boolean {
                return this.$store.state.appStateModule.appState.isDeleteAccountPopupShown;
            }
        },
        components: {
            Button,
            HeaderedInput,
            HeaderlessInput,
            Checkbox,
            DeleteAccountPopup,
        },
    }
)

export default class AccountArea extends Vue {
}

</script>

<style scoped lang="scss">
    .account-area-container {
        padding: 55px 55px 55px 55px;
        position: relative;

        h1 {
            font-family: 'font_bold';
			font-size: 18px;
			line-height: 27px;
            color: #354049;
            margin-block-start: 0.2em;
            margin-block-end: 0.2em;
        }
        h2 {
            font-family: 'font_regular';
			font-size: 16px;
			line-height: 21px;
            color: rgba(56, 75, 101, 0.4);
            margin-block-start: 0.17em;
            margin-block-end: 0.17em;
        }
    }
    .input-container.full-input,
    .input-wrap.full-input {
        width: 100%;
    }
    .mt {
        margin-top: 15px;
    }
    .account-area-settings-container {
        max-width: 680px;
        width: 100%;
        border-radius: 6px;
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: flex-start;
        padding: 32px;
        background-color: #fff;

        &__avatar {
            width: 60px;
            height: 60px;
            border-radius: 6px;
            display: flex;
            align-items: center;
            justify-content: center;
            background: #E8EAF2;
            margin-right: 20px;

            h1 {
                font-family: 'font_medium';
                font-size: 16px;
                line-height: 23px;
                color: #354049;
            }
        }
    }
    .account-area-password-container {
        max-width: 680px;
        width: 100%;
        border-radius: 6px;
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: flex-start;
        padding: 32px;
        background-color: #fff;

        &__svg {
            margin-right: 20px;
        }
    }
    .account-area-button-area {
        margin-top: 40px;
        height: 130px;
    }
    .account-area-row-container__main {
        width: 100%;
        display: flex;
        flex-direction: row;
        align-content: center;
        justify-content: space-between;
        flex-wrap: wrap;
    }
    .account-area-row-container {
        width: 100%;
        display: flex;
        flex-direction: row;
        align-content: center;
        justify-content: flex-start;
    }
    .account-area-save-button-area {
        margin-top: 40px;
        width: 100%;
        align-self: flex-end;
        align-items: center;
        display: flex;
		flex-direction: row;
		justify-content: flex-end;

        Button {
            align-self: center;
        }

        &__btn {
            width: 100%;
            display: flex;
            align-items: center;
            justify-content: flex-end;

            &-active {
                width: 100%;
                display: flex;
                align-items: center;
                justify-content: space-between;
            }
        }
    }

    @media screen and (max-width: 1831px) {
        .account-area-settings-container,
        .account-area-password-container {
            max-width: 530px;
            width: 530px;
        }

        .account-area-save-button-area__cancel-button,
        .account-area-save-button-area__save-button {
            width: 240px !important;
        }
        .account-area-save-button-area__delete-button {
            width: 160px !important;
        }
    }
    @media screen and (max-width: 1520px) {
        .account-area-save-button-area__cancel-button {
            margin-right: 0;
            margin-bottom: 20px;
        }
        .account-area-save-button-area {
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .account-area-button-area {
            margin-bottom: 100px;
        }
        .account-area-save-button-area__btn {
            display: flex;
            align-items: center;
            justify-content: center;
            flex-wrap: wrap;
        }
        .account-area-container {
            overflow-y: scroll;
            height: 800px;
        }
        .account-area-settings-container,
        .account-area-password-container {
            max-width: 450px;
            width: 450px;
            margin-bottom: 30px;
            justify-content: flex-start;
        }
    }
    @media screen and (max-width: 1520px) {
        .account-area-settings-container,
        .account-area-password-container {
            max-width: 450px;
            width: 450px;
            margin-bottom: 30px;
            justify-content: flex-start;
        }
        .account-area-save-button-area__cancel-button,
        .account-area-save-button-area__save-button {
            width: 205px !important;
        }
    }
    @media screen and (max-width: 1330px) {
        .account-area-save-button-area__cancel-button {
            margin-right: 0;
            margin-bottom: 20px;
        }
        .account-area-save-button-area {
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .account-area-button-area {
            margin-bottom: 100px;
        }
        .account-area-save-button-area__btn {
            display: flex;
            align-items: center;
            width: 100%;
            justify-content: flex-end;
            flex-wrap: wrap;
        }
        .account-area-container {
            overflow-y: scroll;
            height: 800px;
        }
        .account-area-settings-container,
        .account-area-password-container {
            max-width: 800px;
            width: 800px;
        }

        .account-area-save-button-area__cancel-button {
            margin-bottom: 0px;
            margin-right: 20px;
        }
        .account-area-save-button-area__cancel-button,
        .account-area-save-button-area__save-button {
            width: 300px !important;
        }
    }
    @media screen and (max-width: 1020px) {
        .account-area-save-button-area {
            flex-direction: column;
            align-items: center;

            &__btn{
                width: 100%;
                justify-content: center;
                margin-top: 40px;
            }
        }
    }
</style>