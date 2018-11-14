<template>
    <div class="aaContainer">
        <!-- TODO: Get info for this area placeholders from store -->
        <!-- TODO: change isDisabled for save buttons for each area when data imputed -->
        <!--start of Account settings area -->
        <div class="aaSettingsContainer">
            <h1>Account Settings</h1>
            <h2>This information will be visible to all users</h2>
            <div class="aaRowContainer">
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
            <div class="aaRowContainer">
                <HeaderedInput 
                    label="Email" 
                    placeholder ="Enter Email" 
                    width="70vw"
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="aaSaveButtonArea">
                <div class="termsArea">
                    <Checkbox class="checkBox" @setData="setTermsAccepted"/>
                    <h2>I agree to the Storj Bridge Hosting <a>Terms & Conditions</a></h2>
                </div>
                <Button label="Save" width="14vw" height="5vh" :onPress="onSave" isWhite isDisabled/> 
            </div>
        </div>
        <!--end of Account settings area -->
        <!--start of Company area -->
        <div class="aaCompanyContainer">
            <h1>Company</h1>
            <h2>Optional</h2>
            <div class="aaRowContainer">
                <HeaderedInput 
                    label="Company Name" 
                    placeholder ="Enter Company Name" 
                    width="70vw"
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="aaRowContainer">
                <HeaderedInput 
                    label="Company Address" 
                    placeholder ="Enter Company Address" 
                    width="70vw"
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="aaRowContainer">
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
            <div class="aaRowContainer">
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
            <div class="aaSaveButtonArea">
                <Button label="Save" width="14vw" height="5vh" :onPress="onSave" isWhite isDisabled/> 
            </div>
        </div>
        <!--end of Company area -->
        <!--start of Password area -->
        <div class="aaPasswordContainer">
            <h1>Change Password</h1>
            <h2>Please choose a password which is longer than 6 characters.</h2>
            <div class="aaRowContainer">
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
            <div class="aaRowContainer">
                <HeaderedInput 
                    label="Confirm password" 
                    placeholder ="Confirm password" 
                    width="70vw"
                    isPassword
                    :error="inputError"
                    @setData="setInputValue" />
            </div>
            <div class="aaSaveButtonArea">
                <Button label="Save" width="14vw" height="5vh" :onPress="onSave" isWhite isDisabled/> 
            </div>
        </div>
        <!--end of Password area -->
        <div class="aaButtonArea">
            <Button label="Delete account" width="14vw" height="5vh" :onPress="onDeleteAccountClick" isWhite/> 
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
    .aaContainer {
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
    .aaSettingsContainer {
        height: 50vh;
        border-radius: 6px;
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: flex-start;
        padding: 3vh 3vh 3vh 3vh;
        background-color: #fff;
    }
    .aaCompanyContainer {
        @extend .aaSettingsContainer;
        margin-top: 5vh;
        height: 75vh;
    }
    .aaPasswordContainer {
        @extend .aaCompanyContainer;
        height: 50vh;
    }
    .aaButtonArea {
        margin-top: 5vh;
        height: 13vh;
    }
    .aaRowContainer {
        width: 100%;
        display: flex;
        flex-direction: row;
        align-content: flex-end;
        justify-content: space-between;
    }
    .aaSaveButtonArea {
        margin-top: 5vh;
        width: 100%;
        align-self: flex-end;
        display: flex;
		flex-direction: row;
		justify-content: flex-end;
        Button {
            align-self: center;
        }
    }
    .termsArea {
		display: flex;
		flex-direction: row;
		justify-content: center;
        align-items: center;
        margin-top: -20px;
        margin-right: 30vw;
		.checkBox {
			align-self: center;
		};
		h2 {
			font-family: 'montserrat_regular';
			font-size: 14px;
			line-height: 20px;
			margin-left: 10px;
		};
		a {
			color: #2683FF;
			font-family: 'montserrat_bold';
		}
	}
</style>