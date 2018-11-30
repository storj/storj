// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

<template>
    <div class="account-area-container">
        <!-- TODO: Get info for this area placeholders from store -->
        <!-- TODO: change isDisabled for save buttons for each area when data imputed -->
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
                    :init-value="cachedFirstName"
                    @setData="setFirstName" />
                <HeaderedInput
                    label="Last Name"
                    placeholder="LastNameEdit"
                    width="100%"
                    ref="lastNameInput"
                    :error="lastNameError"
                    :initValue="cachedLastName"
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
                    :initValue="cachedEmail"
                    @setData="setEmail" />
            </div>
            <div v-if="isAccountSettingsEditing" class="account-area-save-button-area" >
                <!-- v-if we are editing this area -->
                <div class="account-area-save-button-area__terms-area">
                    <Checkbox class="checkbox"
                              @setData="setTermsAccepted"
                              :isCheckboxError="isTermsAcceptedError"/>
                    <h2>I agree to the Storj Bridge Hosting <a>Terms & Conditions</a></h2>
                </div>
                <!-- v-if are editing this area -->
                <div class="account-area-save-button-area__btn">
                    <Button class="account-area-save-button-area__cancel-button" label="Cancel" width="140px"  height="50px" :onPress="onCancelAccountSettingsButtonClick" isWhite/>
                    <Button class="account-area-save-button-area__save-button" label="Save" width="140px"  height="50px" :onPress="onSaveAccountSettingsButtonClick"/>
                </div>
            </div>
            <div v-if="!isAccountSettingsEditing" class="account-area-save-button-area" >
                <!-- v-if we are editing this area -->
                <!-- v-if are editing this area -->
                <div class="account-area-save-button-area__btn">
                    <Button class="account-area-save-button-area__save-button" label="Save" width="140px"  height="50px" :onPress="onSaveAccountSettingsButtonClick" isDisabled />
                </div>
            </div>
        </div>
        <!--end of Account settings area -->
        <!--start of Company area -->
        <div class="account-area-company-container">
            <h1>Company</h1>
            <h2>Optional</h2>
            <div class="account-area-row-container">
                <HeaderedInput
                    class="full-input"
                    label="Company Name"
                    placeholder ="Enter Company Name"
                    width="100%"
                    ref="companyNameInput"
                    :initValue="cachedCompanyName"
                    @setData="setCompanyName" />
            </div>
            <div class="account-area-row-container">
                <HeaderedInput
                    class="full-input"
                    label="Company Address"
                    placeholder ="Enter Company Address"
                    width="100%"
                    ref="companyAddressInput"
                    :initValue="cachedCompanyAddress"
                    @setData="setCompanyAddress" />
            </div>
            <div class="account-area-row-container">
                <HeaderedInput
                    label="Country"
                    placeholder ="Enter Country"
                    width="100%"
                    ref="companyCountryInput"
                    :initValue="cachedCompanyAddress"
                    @setData="setCompanyCountry" />
                <HeaderedInput
                    label="City"
                    placeholder ="Enter City"
                    width="100%"
                    ref="companyCityInput"
                    :initValue="cachedCompanyName"
                    @setData="setCompanyCity" />
            </div>
            <div class="account-area-row-container">
                <HeaderedInput
                    label="State"
                    placeholder ="Enter State"
                    width="100%"
                    ref="companyStateInput"
                    :initValue="cachedCompanyState"
                    @setData="setCompanyState" />
                <HeaderedInput
                    label="Postal Code"
                    placeholder ="Enter Postal Code"
                    width="100%"
                    ref="companyPostalCodeInput"
                    :initValue="cachedCompanyPostalCode"
                    @setData="setCompanyPostalCode" />
            </div>
            <div v-if="isCompanyEditing" class="account-area-save-button-area" >
                <div class="account-area-save-button-area__btn">
                    <!-- v-if we are editing this area -->
                    <Button class="account-area-save-button-area__cancel-button" label="Cancel" width="140px" height="50px" :onPress="onCancelCompanyButtonClick" isWhite/>
                    <Button label="Save" width="140px" height="50px" :onPress="onSaveCompanySettingsButtonClick"/>
                </div>
            </div>
            <div v-if="!isCompanyEditing" class="account-area-save-button-area" >
                <div class="account-area-save-button-area__btn">
                    <!-- v-if we are editing this area -->
                    <Button label="Save" width="140px" height="50px" :onPress="onSaveCompanySettingsButtonClick" isWhite isDisabled/>
                </div>
            </div>
        </div>
        <!--end of Company area -->
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
                    <!-- v-if we are editing this area -->
                    <Button class="account-area-save-button-area__cancel-button" label="Cancel" width="140px" height="50px" :onPress="onCancelPasswordEditButtonClick" isWhite/>
                    <Button label="Save" width="140px" height="50px" :onPress="onSavePasswordButtonClick"/>
                </div>
            </div>
            <div v-if="!isPasswordEditing" class="account-area-save-button-area" >
                <div class="account-area-save-button-area__btn">
                    <!-- v-if we are editing this area -->
                    <Button label="Save" width="140px" height="50px" isWhite isDisabled/>
                </div>
            </div>
        </div>
        <!--end of Password area -->
        <div class="account-area-button-area">
            <Button label="Delete account" width="140px" height="50px" :onPress="onDeleteAccountClick" isWhite/>
        </div>
    </div>
</template>

<script lang="ts">
import { Component, Vue } from 'vue-property-decorator';
import Button from '@/components/common/Button.vue';
import HeaderedInput from '@/components/common/HeaderedInput.vue';
import Checkbox from '@/components/common/Checkbox.vue';

@Component(
    {
        data: function() {
            return {
                cachedFirstName:this.$store.getters.user.firstName,
                cachedLastName: this.$store.getters.user.lastName,
                cachedEmail:this.$store.getters.user.email,

                firstName: this.$store.getters.user.firstName,
                lastName: this.$store.getters.user.lastName,
                email: this.$store.getters.user.email,
                isTermsAccepted: false,

                firstNameError: "",
                lastNameError:"",
                emailError:"",
                isTermsAcceptedError: false,

                newLastName:"",
                newEmail:"",
                isAccountSettingsEditing: false,

                cachedCompanyName: this.$store.getters.user.company.name,
                cachedCompanyAddress: this.$store.getters.user.company.address,
                cachedCompanyCountry: this.$store.getters.user.company.country,
                cachedCompanyCity: this.$store.getters.user.company.city,
                cachedCompanyState: this.$store.getters.user.company.state,
                cachedCompanyPostalCode: this.$store.getters.user.company.postalCode,

                companyName: this.$store.getters.user.company.name,
                companyAddress: this.$store.getters.user.company.address,
                companyCountry: this.$store.getters.user.company.country,
                companyCity: this.$store.getters.user.company.city,
                companyState: this.$store.getters.user.company.state,
                companyPostalCode: this.$store.getters.user.company.postalCode,

                isCompanyEditing: false,

                oldPassword:"",
                newPassword:"",
                confirmationPassword:"",

                oldPasswordError:"",
                newPasswordError:"",
                confirmationPasswordError:"",
                isPasswordEditing: false
            }
        },
        methods: {
            setFirstName: function (value: string) {
                this.$data.firstName = value;
                this.$data.firstNameError = "";
                this.$data.isAccountSettingsEditing = true;
            },
            setLastName: function (value: string) {
                this.$data.lastName = value;
                this.$data.lastNameError = "";
                this.$data.isAccountSettingsEditing = true;
            },
            setEmail: function (value: string) {
                this.$data.email = value;
                this.$data.emailError = "";
                this.$data.isAccountSettingsEditing = true;
            },
            setTermsAccepted: function (value: boolean) {
                this.$data.isTermsAccepted = value;
                this.$data.isTermsAcceptedError = false;
            },
            onCancelAccountSettingsButtonClick: function () {
                this.$data.firstName = this.$data.cachedFirstName;
                this.$data.firstNameError = "";
                this.$data.lastName = this.$store.getters.user.lastName;
                this.$data.lastNameError = "";
                this.$data.email = this.$data.cachedEmail;
                this.$data.emailError = "";

                this.$refs.firstNameInput.setValue(this.$data.cachedFirstName);
                this.$refs.lastNameInput.setValue(this.$data.cachedLastName);
                this.$refs.emailInput.setValue(this.$data.cachedEmail);

                this.$data.isAccountSettingsEditing = false;
            },
            onSaveAccountSettingsButtonClick: async function () {
                let hasError = false;

                if (!this.$data.firstName) {
                    this.$data.firstNameError = "First name expected";
                    hasError = true;
                }

                if (!this.$data.lastName) {
                    this.$data.lastNameError = "Last name expected";
                    hasError = true;
                }

                if (!this.$data.email) {
                    this.$data.emailError = "Email expected";
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
                await this.$store.dispatch("updateBasicUserInfo", user);
                this.$data.isAccountSettingsEditing = false;
            },

            setCompanyName:function (value: string) {
                this.$data.companyName = value;
                this.$data.isCompanyEditing = true;
            },
            setCompanyAddress:function (value: string) {
                this.$data.companyAddress = value;
                this.$data.isCompanyEditing = true;
            },
            setCompanyCountry: function (value: string) {
                this.$data.companyCountry = value;
                this.$data.isCompanyEditing = true;
            },
            setCompanyCity:function (value: string) {
                this.$data.companyCity = value;
                this.$data.isCompanyEditing = true;
            },
            setCompanyState:function (value: string) {
                this.$data.companyState = value;
                this.$data.isCompanyEditing = true;
            },
            setCompanyPostalCode:function (value: string) {
                this.$data.companyPostalCode = value;
                this.$data.isCompanyEditing = true;
            },
            onCancelCompanyButtonClick: function () {
                this.$data.companyName=this.$data.cachedCompanyName;
                this.$data.companyAddress=this.$data.cachedCompanyAddress;
                this.$data.companyCountry=this.$data.cachedCompanyCountry;
                this.$data.companyCity=this.$data.cachedCompanyCity;
                this.$data.companyState=this.$data.cachedCompanyState;
                this.$data.companyPostalCode=this.$data.cachedCompanyPostalCode;

                this.$refs.companyNameInput.setValue(this.$data.cachedCompanyName);
                this.$refs.companyAddressInput.setValue(this.$data.cachedCompanyAddress);
                this.$refs.companyCountryInput.setValue(this.$data.cachedCompanyCountry);
                this.$refs.companyCityInput.setValue(this.$data.cachedCompanyCity);
                this.$refs.companyStateInput.setValue(this.$data.cachedCompanyState);
                this.$refs.companyPostalCodeInput.setValue(this.$data.cachedCompanyPostalCode);

                this.$data.isCompanyEditing= false;
            },
            onSaveCompanySettingsButtonClick: async function () {
                let user = {
                    id: this.$store.getters.user.id,
                    company : {
                        name: this.$data.companyName,
                        address: this.$data.companyAddress,
                        country: this.$data.companyCountry,
                        city: this.$data.companyCity,
                        state: this.$data.companyState,
                        postalCode: this.$data.companyPostalCode
                    }
                };

                await this.$store.dispatch("updateCompanyInfo", user);
                this.$data.isCompanyEditing = false;
            },

            setOldPassword: function (value: string) {
                this.$data.oldPassword = value;
                this.$data.oldPasswordError = "";
                this.$data.isPasswordEditing = true;
            },
            setNewPassword: function (value: string) {
                this.$data.newPassword = value;
                this.$data.newPasswordError = "";
                this.$data.isPasswordEditing = true;
            },
            setPasswordConfirmation: function (value: string) {
                this.$data.confirmationPassword = value;
                this.$data.confirmationPasswordError = "";
                this.$data.isPasswordEditing = true;
            },
            onCancelPasswordEditButtonClick: function () {
                this.$data.oldPassword = "";
                this.$data.newPassword = "";
                this.$data.confirmationPassword = "";

                this.$data.oldPasswordError = "";
                this.$data.newPasswordError = "";
                this.$data.confirmationPasswordError = "";

                this.$refs.oldPasswordInput.setValue("");
                this.$refs.newPasswordInput.setValue("");
                this.$refs.confirmationPasswordInput.setValue("");

                this.$data.isPasswordEditing = false;
            },
            onSavePasswordButtonClick: async function () {
                let hasError = false;

                if (!this.$data.oldPassword) {
                    this.$data.oldPasswordError = "Password required";
                    hasError = true;
                }

                if(!this.$data.newPassword) {
                    this.$data.newPasswordError = "Password required";
                    hasError = true;
                }

                if (!this.$data.confirmationPassword) {
                    this.$data.confirmationPasswordError = "Password required";
                    hasError = true;
                }

                if(this.$data.newPassword !== this.$data.confirmationPassword) {
                    this.$data.confirmationPasswordError = "Password not match to new one";
                    hasError = true;
                }

                if (hasError) {
                    return;
                }

                await this.$store.dispatch("updatePassword",this.$data.newPassword);

                this.$refs.oldPasswordInput.setValue("");
                this.$refs.newPasswordInput.setValue("");
                this.$refs.confirmationPasswordInput.setValue("");

                this.$data.isPasswordEditing = false;
            },
            onDeleteAccountClick: async function () {
                // TODO show popup with user confirmation
                await this.$store.dispatch("deleteUserAccount");

                this.$refs.oldPasswordInput.setValue("");
                this.$refs.newPasswordInput.setValue("");
                this.$refs.confirmationPasswordInput.setValue("");
            }
        },
        components: {
            Button,
            HeaderedInput,
            Checkbox
        },
        computed: {
        }
    }
)

export default class AccountArea extends Vue {}

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

    .input-container.full-input {
        width: 100%;
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