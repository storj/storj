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
                    :error="inputError"
                    @setData="setInputValue" />
                <HeaderedInput 
                    label="Last Name" 
                    width="100%"
                    placeholder ="Enter Last Name" 
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-row-container">
                <HeaderedInput
                    class="full-input"
                    label="Email"
                    placeholder ="Enter Email" 
                    width="100%"
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-save-button-area">
                <!-- v-if we are editing this area -->
                <div class="account-area-save-button-area__terms-area">
                    <Checkbox class="checkbox" @setData="setTermsAccepted"/>
                    <h2>I agree to the Storj Bridge Hosting <a>Terms & Conditions</a></h2>
                </div>
                <!-- v-if are editing this area -->
                <div class="account-area-save-button-area__btn">
                    <Button class="account-area-save-button-area__cancel-button" label="Cancel" width="140px"  height="50px" :onPress="onCancel" isWhite/>
                    <Button class="account-area-save-button-area__save-button" label="Save" width="140px"  height="50px" :onPress="onSave" isDisabled/>     
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
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-row-container">
                <HeaderedInput
                    class="full-input"
                    label="Company Address" 
                    placeholder ="Enter Company Address" 
                    width="100%"
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-row-container">
                <HeaderedInput 
                    label="Country" 
                    placeholder ="Enter Country" 
                    width="100%"
                    :error="inputError"
                    @setData="setInputValue" />
                <HeaderedInput 
                    label="City" 
                    placeholder ="Enter City" 
                    width="100%"
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-row-container">
                <HeaderedInput 
                    label="State" 
                    placeholder ="Enter State" 
                    width="100%"
                    :error="inputError"
                    @setData="setInputValue" />
                <HeaderedInput 
                    label="Postal Code" 
                    placeholder ="Enter Postal Code" 
                    width="100%"
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-save-button-area__btn">
                <!-- v-if we are editing this area -->
                <Button class="account-area-save-button-area__cancel-button" label="Cancel" width="140px" height="50px" :onPress="onCancel" isWhite/>
                <Button label="Save" width="140px" height="50px" :onPress="onSave" isWhite isDisabled/> 
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
                    :error="inputError"
                    @setData="setInputValue" />
                <HeaderedInput 
                    label="New Password" 
                    placeholder ="Enter New Password" 
                    width="100%"
                    isPassword
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-row-container">
                <HeaderedInput 
                    class="full-input"
                    label="Confirm password" 
                    placeholder ="Confirm password" 
                    width="100%"
                    isPassword
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-save-button-area__btn">
                <!-- v-if we are editing this area -->
                <Button class="account-area-save-button-area__cancel-button" label="Cancel" width="140px" height="50px" :onPress="onCancel" isWhite/>
                <Button label="Save" width="140px" height="50px" :onPress="onSave" isWhite isDisabled/> 
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

            }
        },
        components: {
            Button,
            HeaderedInput,
            Checkbox
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