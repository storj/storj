// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-area-container">
        <!--start of Account settings area -->
        <div class="account-area-settings-container">
            <h1>Account Settings</h1>
            <h2>This information will be visible to all users</h2>
            <div class="account-area-row-container">
                <HeaderedInput
                    label="First name"
                    placeholder ="Enter First Name"
                    width="100%"
                    ref="firstNameInput"
                    :error="firstNameError"
                    :init-value="originalFirstName"
                    @setData="setFirstName" />
                <HeaderedInput
                    label="Last Name"
                    placeholder="LastNameEdit"
                    width="100%"
                    ref="lastNameInput"
                    :error="lastNameError"
                    :initValue="originalLastName"
                    @setData="setLastName"/>
            </div>
            <div class="account-area-row-container">
                <HeaderedInput
                    class="full-input"
                    label="Email"
                    placeholder ="Enter Email"
                    width="100%"
                    ref="emailInput"
                    :error="emailError"
                    :initValue="originalEmail"
                    @setData="setEmail" />
            </div>
            <div v-if="isAccountSettingsEditing" class="account-area-save-button-area" >
                <div class="account-area-save-button-area__terms-area">
                    <Checkbox class="checkbox"
                              @setData="setTermsAccepted"
                              :isCheckboxError="isTermsAcceptedError"/>
                    <h2>I agree to the Storj Bridge Hosting <a>Terms & Conditions</a></h2>
                </div>
                <div class="account-area-save-button-area__btn">
                    <Button class="account-area-save-button-area__cancel-button" label="Cancel" width="140px"  height="50px" :onPress="onCancelAccountSettingsButtonClick" isWhite/>
                    <Button class="account-area-save-button-area__save-button" label="Save" width="140px"  height="50px" :onPress="onSaveAccountSettingsButtonClick"/>
                </div>
            </div>
            <div v-if="!isAccountSettingsEditing" class="account-area-save-button-area" >
                <div class="account-area-save-button-area__btn">
                    <Button class="account-area-save-button-area__save-button" label="Save" width="140px"  height="50px" :onPress="onSaveAccountSettingsButtonClick" isDisabled />
                </div>
            </div>
        </div>
        <!--end of Account settings area -->
        <!--start of Password area -->
        <div class="account-area-password-container">
            <h1>Change Password</h1>
            <h2>Please choose a password which is longer than 6 characters.</h2>
            <div class="account-area-row-container">
                <HeaderedInput
                    label="Old Password"
                    placeholder ="Enter Old Password"
                    width="100%"
                    isPassword
                    ref="oldPasswordInput"
                    :error="oldPasswordError"
                    @setData="setOldPassword" />
                <HeaderedInput
                    label="New Password"
                    placeholder ="Enter New Password"
                    width="100%"
                    ref="newPasswordInput"
                    isPassword
                    :error="newPasswordError"
                    @setData="setNewPassword" />
            </div>
            <div class="account-area-row-container">
                <HeaderedInput
                    class="full-input"
                    label="Confirm password"
                    placeholder ="Confirm password"
                    width="100%"
                    ref="confirmationPasswordInput"
                    isPassword
                    :error="confirmationPasswordError"
                    @setData="setPasswordConfirmation" />
            </div>
            <div v-if="isPasswordEditing" class="account-area-save-button-area" >
                <div class="account-area-save-button-area__btn">
                    <Button class="account-area-save-button-area__cancel-button" label="Cancel" width="140px" height="50px" :onPress="onCancelPasswordEditButtonClick" isWhite/>
                    <Button label="Save" width="140px" height="50px" :onPress="onSavePasswordButtonClick"/>
                </div>
            </div>
            <div v-if="!isPasswordEditing" class="account-area-save-button-area" >
                <div class="account-area-save-button-area__btn">
                    <Button label="Save" width="140px" height="50px" isWhite isDisabled/>
                </div>
            </div>
        </div>
        <!--end of Password area -->
        <div class="account-area-button-area">
            <Button label="Delete account" width="140px" height="50px" :onPress="togglePopup" isWhite/>
        </div>
        <DeleteAccountPopup v-if="isPopupShown" :onClose="togglePopup" />
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import Button from '@/components/common/Button.vue';
import HeaderedInput from '@/components/common/HeaderedInput.vue';
import Checkbox from '@/components/common/Checkbox.vue';
import ROUTES from '@/utils/constants/routerConstants';
import DeleteAccountPopup from '@/components/dashboard/account/DeleteAccountPopup.vue';

@Component(
    {
        data: function () {
            return {
                originalFirstName: this.$store.getters.user.firstName,
                originalLastName: this.$store.getters.user.lastName,
                originalEmail: this.$store.getters.user.email,

                firstName: this.$store.getters.user.firstName,
                lastName: this.$store.getters.user.lastName,
                email: this.$store.getters.user.email,
                isTermsAccepted: false,

                firstNameError: '',
                lastNameError: '',
                emailError: '',
                isTermsAcceptedError: false,

                newLastName: '',
                newEmail: '',
                isAccountSettingsEditing: false,

                oldPassword: '',
                newPassword: '',
                confirmationPassword: '',

                oldPasswordError: '',
                newPasswordError: '',
                confirmationPasswordError: '',
                isPasswordEditing: false,

                isPopupShown: false
            };
        },
        methods: {
            setFirstName: function (value: string) {
                this.$data.firstName = value;
                this.$data.firstNameError = '';
                this.$data.isAccountSettingsEditing = true;
            },
            setLastName: function (value: string) {
                this.$data.lastName = value;
                this.$data.lastNameError = '';
                this.$data.isAccountSettingsEditing = true;
            },
            setEmail: function (value: string) {
                this.$data.email = value;
                this.$data.emailError = '';
                this.$data.isAccountSettingsEditing = true;
            },
            setTermsAccepted: function (value: boolean) {
                this.$data.isTermsAccepted = value;
                this.$data.isTermsAcceptedError = false;
            },
            onCancelAccountSettingsButtonClick: function () {
                this.$data.firstName = this.$data.originalFirstName;
                this.$data.firstNameError = '';
                this.$data.lastName = this.$store.getters.user.lastName;
                this.$data.lastNameError = '';
                this.$data.email = this.$data.originalEmail;
                this.$data.emailError = '';

                (this.$refs['firstNameInput'] as HeaderedInput).setValue(this.$data.originalFirstName);
                (this.$refs['lastNameInput'] as HeaderedInput).setValue(this.$data.originalLastName);
                (this.$refs['emailInput'] as HeaderedInput).setValue(this.$data.originalEmail);

                this.$data.isAccountSettingsEditing = false;
            },
            onSaveAccountSettingsButtonClick: async function () {
                let hasError = false;

                if (!this.$data.firstName) {
                    this.$data.firstNameError = 'First name expected';
                    hasError = true;
                }

                if (!this.$data.lastName) {
                    this.$data.lastNameError = 'Last name expected';
                    hasError = true;
                }

                if (!this.$data.email) {
                    this.$data.emailError = 'Email expected';
                    hasError = true;
                }

                if (!this.$data.isTermsAccepted) {
                    this.$data.isTermsAcceptedError = true;
                    hasError = true;
                }

                if (hasError) {
                    return;
                }

                let user = {
                    id: this.$store.getters.user.id,
                    email: this.$data.email,
                    firstName: this.$data.firstName,
                    lastName: this.$data.lastName,
                };
                let isSuccess = await this.$store.dispatch('updateBasicUserInfo', user);
                if (!isSuccess) {
                    // TODO Change to popup
                    console.error('error while changing basic user info');

                    return;
                }
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

                (this.$refs['oldPasswordInput'] as HeaderedInput).setValue('');
                (this.$refs['newPasswordInput'] as HeaderedInput).setValue('');
                (this.$refs['confirmationPasswordInput'] as HeaderedInput).setValue('');

                this.$data.isPasswordEditing = false;
            },
            onSavePasswordButtonClick: async function () {
                let hasError = false;

                if (!this.$data.oldPassword) {
                    this.$data.oldPasswordError = 'Password required';
                    hasError = true;
                }

                if (!this.$data.newPassword) {
                    this.$data.newPasswordError = 'Password required';
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

                let isSuccess = await this.$store.dispatch('updatePassword', this.$data.newPassword);
                if (!isSuccess) {
                    // TODO Change to popup
                    console.error('error while updating user password');

                    return;
                }

                (this.$refs['oldPasswordInput'] as HeaderedInput).setValue('');
                (this.$refs['newPasswordInput'] as HeaderedInput).setValue('');
                (this.$refs['confirmationPasswordInput'] as HeaderedInput).setValue('');

                this.$data.isPasswordEditing = false;
            },
            togglePopup: function(): void {
                this.$data.isPopupShown = ! this.$data.isPopupShown;
            },
        },
        components: {
            Button,
            HeaderedInput,
            Checkbox,
            DeleteAccountPopup
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
        overflow-y: auto;
        overflow-x: hidden;
        height: 80vh;
        h1 {
            font-family: 'montserrat_bold';
			font-size: 18px;
			line-height: 27px;
            color: #354049;
        }
        h2 {
            font-family: 'montserrat_regular';
			font-size: 16px;
			line-height: 21px;
            color: rgba(56, 75, 101, 0.4);
        }
    }
    .account-area-settings-container {
        height: 50vh;
        border-radius: 6px;
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: flex-start;
        padding: 32px;
        background-color: #fff;
    }
    .account-area-company-container {
        @extend .account-area-settings-container;
        margin-top: 40px;
        height: 75vh;
    }
    .account-area-password-container {
        @extend .account-area-company-container;
        height: 50vh;
    }
    .account-area-button-area {
        margin-top: 40px;
        height: 130px;
    }
    .account-area-row-container {
        width: 100%;
        display: flex;
        flex-direction: row;
        align-content: center;
        justify-content: space-between;
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
            display: flex;
            align-items: center;
        }

        &__terms-area {
            display: flex;
            flex-direction: row;
            justify-content: flex-start;
            align-items: center;
            width: 100%;

            .checkbox {
                align-self: center;
            };
            h2 {
                font-family: 'montserrat_regular';
                font-size: 14px;
                line-height: 20px;
                margin-left: 10px;
                margin-top: 30px;
            };

            a {
                color: #2683FF;
                font-family: 'montserrat_bold';

                &:hover {
                    text-decoration: underline;
                }
            }
        }

        &__cancel-button {
            margin-right: 20px;
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

            &__terms-area{
                justify-content: center;
                margin-bottom: 20px;
            }
        }
    }
</style>