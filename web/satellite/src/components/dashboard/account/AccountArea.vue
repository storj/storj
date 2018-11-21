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
                    width="33vw"
                    :error="inputError"
                    @setData="setInputValue" />
                <HeaderedInput 
                    label="Last Name" 
                    width="33vw"
                    placeholder ="Enter Last Name" 
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-row-container">
                <HeaderedInput 
                    label="Email" 
                    placeholder ="Enter Email" 
                    width="70vw"
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-save-button-area">
                <!-- v-if we are editing this area -->
                <div class="account-area-save-button-area__terms-area">
                    <Checkbox class="checkbox" @setData="setTermsAccepted"/>
                    <h2>I agree to the Storj Bridge Hosting <a>Terms & Conditions</a></h2>
                </div>
                <!-- v-if we are editing this area -->
                <Button class="account-area-save-button-area__cancel-button" label="Cancel" width="10vw" height="5vh" :onPress="onCancel" isWhite/>
                <Button label="Save" width="10vw" height="5vh" :onPress="onSave" isDisabled/> 
            </div>
        </div>
        <!--end of Account settings area -->
        <!--start of Company area -->
        <div class="account-area-company-container">
            <h1>Company</h1>
            <h2>Optional</h2>
            <div class="account-area-row-container">
                <HeaderedInput 
                    label="Company Name" 
                    placeholder ="Enter Company Name" 
                    width="70vw"
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-row-container">
                <HeaderedInput 
                    label="Company Address" 
                    placeholder ="Enter Company Address" 
                    width="70vw"
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-row-container">
                <HeaderedInput 
                    label="Country" 
                    placeholder ="Enter Country" 
                    width="33vw"
                    :error="inputError"
                    @setData="setInputValue" />
                <HeaderedInput 
                    label="City" 
                    placeholder ="Enter City" 
                    width="33vw"
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-row-container">
                <HeaderedInput 
                    label="State" 
                    placeholder ="Enter State" 
                    width="33vw"
                    :error="inputError"
                    @setData="setInputValue" />
                <HeaderedInput 
                    label="Postal Code" 
                    placeholder ="Enter Postal Code" 
                    width="33vw"
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-save-button-area">
                <!-- v-if we are editing this area -->
                <Button class="account-area-save-button-area__cancel-button" label="Cancel" width="10vw" height="5vh" :onPress="onCancel" isWhite/>
                <Button label="Save" width="10vw" height="5vh" :onPress="onSave" isWhite isDisabled/> 
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
                    width="33vw"
                    isPassword
                    :error="inputError"
                    @setData="setInputValue" />
                <HeaderedInput 
                    label="New Password" 
                    placeholder ="Enter New Password" 
                    width="33vw"
                    isPassword
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-row-container">
                <HeaderedInput 
                    label="Confirm password" 
                    placeholder ="Confirm password" 
                    width="70vw"
                    isPassword
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="account-area-save-button-area">
                <!-- v-if we are editing this area -->
                <Button class="account-area-save-button-area__cancel-button" label="Cancel" width="10vw" height="5vh" :onPress="onCancel" isWhite/>
                <Button label="Save" width="10vw" height="5vh" :onPress="onSave" isWhite isDisabled/> 
            </div>
        </div>
        <!--end of Password area -->
        <div class="account-area-button-area">
            <Button label="Delete account" width="10vw" height="5vh" :onPress="onDeleteAccountClick" isWhite/> 
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
        padding: 3vh 3vh 3vh 3vh;
        background-color: #fff;
    }
    .account-area-company-container {
        @extend .account-area-settings-container;
        margin-top: 5vh;
        height: 75vh;
    }
    .account-area-password-container {
        @extend .account-area-company-container;
        height: 50vh;
    }
    .account-area-button-area {
        margin-top: 5vh;
        height: 13vh;
    }
    .account-area-row-container {
        width: 100%;
        display: flex;
        flex-direction: row;
        align-content: flex-end;
        justify-content: space-between;
    }
    .account-area-save-button-area {
        margin-top: 5vh;
        width: 100%;
        align-self: flex-end;
        display: flex;
		flex-direction: row;
		justify-content: flex-end;

        Button {
            align-self: center;
        }

        &__terms-area {
            display: flex;
            flex-direction: row;
            justify-content: center;
            align-items: center;
            margin-top: -1vh;
            margin-right: 22vw;
            .checkbox {
                align-self: center;
            };
            h2 {
                font-family: 'montserrat_regular';
                font-size: 14px;
                line-height: 20px;
                margin-left: 10px;
                margin-top: 3vh;
            };
            a {
                color: #2683FF;
                font-family: 'montserrat_bold';
            }
        }

        &__cancel-button {
            margin-right: 2vw;
        }
    }
</style>